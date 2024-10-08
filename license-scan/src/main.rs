#![deny(rust_2018_idioms)]
#![warn(clippy::pedantic)]
#![allow(clippy::redundant_closure_for_method_calls)]

use anyhow::{anyhow, bail, ensure, Context, Result};
use askalono::{ScanStrategy, Store, TextData};
use ignore::types::{Types, TypesBuilder};
use ignore::WalkBuilder;
use semver::VersionReq;
use serde::{Deserialize, Deserializer};
use spdx::Expression;
use std::cmp::Ordering;
use std::collections::{BTreeMap, HashMap};
use std::convert::TryInto;
use std::fmt;
use std::fs;
use std::hash::Hasher;
use std::path::{Path, PathBuf};
use structopt::StructOpt;
use walkdir::WalkDir;

#[derive(Debug, StructOpt)]
struct Opt {
    /// An optional clarification file.
    #[structopt(long)]
    clarify: Option<PathBuf>,

    /// Path to the SPDX license data (json/details in license-list-data)
    #[structopt(long)]
    spdx_data: PathBuf,

    /// Where to write attribution.txt and copies of license files.
    #[structopt(long)]
    out_dir: PathBuf,

    #[structopt(subcommand)]
    cmd: Cmd,
}

#[derive(Debug, StructOpt)]
enum Cmd {
    GoVendor {
        /// Path to the vendor directory of a project.
        vendor_dir: PathBuf,
    },
    Cargo {
        /// Path to Cargo.toml for a project.
        manifest_path: PathBuf,

        /// Equivalent to `cargo --locked`
        #[structopt(long)]
        locked: bool,

        /// Equivalent to `cargo --offline`
        #[structopt(long)]
        offline: bool,
    },
}

fn main() -> Result<()> {
    println!("Hello, world!");
    let opt = Opt::from_args();

    let clarify = match opt.clarify {
        None => Clarifications::default(),
        Some(path) => toml::from_str(&fs::read_to_string(path)?)?,
    };

    let mut store = Store::new();
    store.load_spdx(&opt.spdx_data, false)?;
    let scanner = ScanStrategy::new(&store)
        .confidence_threshold(0.93)
        .shallow_limit(1.0)
        .optimize(true);

    match opt.cmd {
        Cmd::GoVendor { vendor_dir } => {
            for repo in scan_go_vendor_repos(&vendor_dir)? {
                write_attribution(
                    repo.to_str().with_context(|| {
                        format!(
                            "package name is not valid UTF-8; lossy version is '{}'",
                            repo.to_string_lossy()
                        )
                    })?,
                    None,
                    &vendor_dir.join(&repo),
                    &opt.out_dir.join(&repo),
                    &scanner,
                    &clarify,
                    None,
                )?;
            }
            Ok(())
        }
        Cmd::Cargo {
            manifest_path,
            locked,
            offline,
        } => {
            let mut builder = cargo_metadata::MetadataCommand::new();
            builder.manifest_path(manifest_path);
            if locked {
                builder.other_options(vec!["--locked".to_owned()]);
            }
            if offline {
                builder.other_options(vec!["--offline".to_owned()]);
            }
            let metadata = builder.exec()?;
            for package in metadata.packages {
                if package.source.is_none() {
                    if let Some(publish) = package.publish {
                        if publish.is_empty() {
                            // `package.source` is None if the project is a local project;
                            // `package.publish` is an empty Vec if `publish = false` is set
                            continue;
                        }
                    }
                }
                write_attribution(
                    &package.name,
                    Some(&package.version.to_string().parse()?),
                    package
                        .manifest_path
                        .into_std_path_buf()
                        .parent()
                        .expect("expected a path to Cargo.toml to have a parent"),
                    &opt.out_dir
                        .join(format!("{}-{}", package.name, package.version)),
                    &scanner,
                    &clarify,
                    if let Some(license) = package.license {
                        Some(Expression::parse(&unslash(&license)).map_err(|err| {
                            // spdx errors use the lifetime of the string
                            anyhow!(err.to_string())
                        })?)
                    } else {
                        None
                    },
                )?;
            }
            Ok(())
        }
    }
}

#[derive(Debug, Deserialize, Default)]
struct Clarifications {
    #[serde(default)]
    clarify: HashMap<String, Clarification>,
}

/// A clarification for a package overrides the auto-detected license string.
///
/// It can be used in situations where the detected license is incorrect (for example, because it's
/// difficult for a computer to tell the difference between "MIT AND Apache-2.0" and "MIT OR
/// Apache-2.0").
///
/// It *must* be used in situations where we can't determine with reasonable confidence what a
/// license file matches to, or if we need to skip files that look like they could be licenses.
///
/// This program gets a list of all license files and their hashes; the file list must match the
/// list in the clarification (or missing files must be in `skip_files`), or it will return an
/// error to ensure changes to license information for a package is inspected.
#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum Clarification {
    SingleClarification(Box<InnerClarification>),
    ClarificationList(Vec<InnerClarification>),
}

impl Clarification {
    fn matches(&self, query_version: Option<&semver::Version>) -> Option<&InnerClarification> {
        match self {
            Self::SingleClarification(c) => c.matches_version(query_version).then_some(c),
            Self::ClarificationList(cl) => cl
                .iter()
                .find(|clarification| clarification.matches_version(query_version)),
        }
    }
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
struct InnerClarification {
    /// The optional version constraint. Defaults to any version.
    version: Option<VersionReq>,

    /// The SPDX license expression for the entire package.
    #[serde(deserialize_with = "expression_from_str")]
    expression: Expression,

    /// List of files containing license information and their hashes.
    license_files: Vec<LicenseFile>,

    /// List of files that should be skipped as they don't contain license information.
    #[serde(default)]
    skip_files: Vec<PathBuf>,
}

impl InnerClarification {
    /// Checks whether this clarification matches an input version.
    ///
    /// If the clarification has no version, it matches any input version.
    /// If the clarification has a version, but the query version is empty, it does not match.
    fn matches_version(&self, query_version: Option<&semver::Version>) -> bool {
        match (&self.version, query_version) {
            (Some(version), Some(query_version)) => version.matches(query_version),
            (Some(_), None) => false,
            (None, _) => true,
        }
    }
}

#[derive(Debug, Deserialize)]
struct LicenseFile {
    path: String,
    hash: u32,
}

#[derive(Debug, PartialEq)]
struct Clarified<'a> {
    expression: &'a Expression,
    skip_files: &'a Vec<PathBuf>,
}

impl Clarifications {
    /// Gets a clarification for a package.
    ///
    /// If a clarification isn't present for that name, `Ok(None)` is returned.
    ///
    /// If a clarification is present and the file list matches, `Ok(Some(Clarified))` is returned.
    ///
    /// If a clarification is present and the file list does not match, `Err(_)` is returned.
    fn get(
        &self,
        name: &str,
        version: Option<&semver::Version>,
        mut files: BTreeMap<&Path, u32>,
    ) -> Result<Option<Clarified<'_>>> {
        if let Some(clarification) = self
            .clarify
            .get(name)
            .and_then(|clarification| clarification.matches(version))
        {
            // first remove files to skip
            for file in &clarification.skip_files {
                files.remove(file.as_path());
            }

            // convert `clarification.license_files` into a struct we can compare with `files`
            let clarify_files = clarification
                .license_files
                .iter()
                .map(|file| (Path::new(&file.path), file.hash))
                .collect::<BTreeMap<_, _>>();
            ensure!(
                files == clarify_files,
                "file mismatch in clarification for {}\nclarification: {:#x?}\nscanned: {:#x?}",
                name,
                clarify_files,
                files,
            );
            Ok(Some(Clarified {
                expression: &clarification.expression,
                skip_files: &clarification.skip_files,
            }))
        } else {
            Ok(None)
        }
    }
}

/// `#[serde(deserialize_with)]` handler for parsing as an `spdx::Expression`.
fn expression_from_str<'de, D>(deserializer: D) -> Result<Expression, D::Error>
where
    D: Deserializer<'de>,
{
    struct Visitor;

    impl<'de> serde::de::Visitor<'de> for Visitor {
        type Value = Expression;

        fn expecting(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
            formatter.write_str("a string")
        }

        fn visit_str<E>(self, s: &str) -> Result<Self::Value, E>
        where
            E: serde::de::Error,
        {
            Expression::parse(s).map_err(|err| E::custom(err.to_string()))
        }
    }

    deserializer.deserialize_str(Visitor)
}

lazy_static::lazy_static! {
    static ref TYPES: Types = {
        let mut builder = TypesBuilder::new();
        // there's a package with a "License" file and that isn't covered in ignore::types
        builder.add("moarlicense", "License").unwrap();
        builder.add_defaults()
            .select("license")
            .select("moarlicense")
            .build()
            .unwrap()
    };
}

/// Replace '/' characters in a license string with 'OR'. (crates.io allows '/' instead of 'OR' for
/// compatibility.)
fn unslash(s: &str) -> String {
    s.split('/').map(str::trim).collect::<Vec<_>>().join(" OR ")
}

/// Returns true if the file is expected to not be a license text (such as the Apache-2.0 NOTICE
/// file).
fn non_license(path: &Path) -> bool {
    match path.file_name().and_then(|s| s.to_str()) {
        Some(file_name) => file_name.starts_with("NOTICE") || file_name.starts_with("PATENTS"),
        None => false,
    }
}

#[allow(clippy::cast_possible_truncation)]
fn hash(data: &[u8]) -> u32 {
    let mut hasher = twox_hash::XxHash32::default();
    hasher.write(data);
    hasher
        .finish()
        .try_into()
        .expect("XxHash32 returned hash larger than 32 bits")
}

#[allow(clippy::too_many_lines)] // maybe someday...
fn write_attribution(
    name: &str,
    version: Option<&semver::Version>,
    scan_dir: &Path,
    out_dir: &Path,
    scanner: &ScanStrategy<'_>,
    clarifications: &Clarifications,
    stated_license: Option<Expression>,
) -> Result<()> {
    eprintln!("{name}:");
    if let Some(stated_license) = stated_license.as_ref() {
        eprintln!("  + {stated_license} (stated in metadata)");
    }
    let mut files = HashMap::new();
    for entry in WalkBuilder::new(scan_dir).types(TYPES.clone()).build() {
        let entry = entry?;
        if entry.file_type().map_or(false, |ft| ft.is_file()) {
            let rel_path = entry.path().strip_prefix(scan_dir)?;
            let data = fs::read_to_string(entry.path())
                .with_context(|| format!("failed to read {}", entry.path().display()))?;
            let file_hash = hash(data.as_bytes());
            files.insert(rel_path.to_owned(), (data, file_hash));
        }
    }

    let file_hashes = files
        .iter()
        .map(|(file, (_, hash))| (file.as_path(), *hash))
        .collect();
    let license = if let Some(clarified) = clarifications.get(name, version, file_hashes)? {
        let expression = clarified.expression.to_string();
        eprintln!("  ! {expression} (clarified)");
        copy_files(out_dir, &files, clarified.skip_files)?;
        expression
    } else {
        let mut licenses = Vec::new();
        for (file, (data, file_hash)) in &files {
            let containing = scanner.scan(&TextData::new(data))?.containing;
            if containing.is_empty() {
                if non_license(file) {
                    eprintln!(
                        "  + {} (hash = 0x{:x}) detected as non-license file",
                        file.display(),
                        file_hash
                    );
                } else {
                    if stated_license.is_some() {
                        // if the package states a license and we heuristically detect that this is
                        // a top-level "either license, at your option" file, ignore it
                        let trainwreck = data.split_whitespace().collect::<Vec<_>>().join(" ");
                        if trainwreck.contains("under the terms of either license")
                            || trainwreck.contains("at your option")
                        {
                            eprintln!(
                                "  + {} (hash = 0x{:x}) detected as non-license file",
                                file.display(),
                                file_hash
                            );
                            continue;
                        }
                    }
                    bail!(
                        "failed to detect any license from {} (hash = 0x{:x}), \
                         please add a clarification",
                        scan_dir.join(file).display(),
                        file_hash,
                    );
                }
            }
            for result in containing {
                eprintln!(
                    "  + {} (hash = 0x{:x}) detected as {} (confidence {:.4})",
                    file.display(),
                    file_hash,
                    result.license.name,
                    result.score,
                );
                if let Some(stated_license) = stated_license.as_ref() {
                    // The license we detected should be included in the stated license string,
                    // otherwise we know the stated license is incomplete, in which case we should
                    // have had a clarification.
                    ensure!(
                        stated_license.requirements().any(|er| {
                            // `er` is an `ExpressionReq`; `er.req` is a `LicenseReq`.
                            // `er.req.license.id()` returns `Option<LicenseId>`.
                            er.req.license.id().is_some()
                                && er.req.license.id() == spdx::license_id(result.license.name)
                        }),
                        "detected license \"{}\" from {} is not present in the license \
                         field \"{}\" for {}",
                        result.license.name,
                        file.display(),
                        stated_license,
                        name
                    );
                } else {
                    licenses.push(result.license.name);
                }
            }
        }

        copy_files(out_dir, &files, &[])?;

        if let Some(stated_license) = stated_license {
            stated_license.to_string()
        } else {
            licenses.sort_unstable();
            licenses.dedup();
            let expression = licenses.join(" AND ");
            eprintln!("  = {expression}");
            expression
        }
    };

    fs::create_dir_all(out_dir)?;
    fs::write(
        out_dir.join("attribution.txt"),
        format!("{name}\nSPDX-License-Identifier: {license}\n"),
    )?;
    Ok(())
}

fn copy_files(
    out_dir: &Path,
    files: &HashMap<PathBuf, (String, u32)>,
    skip_files: &[PathBuf],
) -> Result<()> {
    for (file, (data, _)) in files {
        if skip_files.contains(file) {
            continue;
        }

        let path = out_dir.join(file);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        fs::write(path, data)?;
    }
    Ok(())
}

/// Searches for repositories in a Go vendor directory.
///
/// This finds the shallowest directories that contain files. The logic here is the directory
/// structure leading up to a module won't contain any files, but as soon as you reach a repository
/// root, it will *probably* contain a file.
///
/// This ignores files at the top level of the vendor directory (such as `go mod`'s `modules.txt`
/// file).
fn scan_go_vendor_repos(vendor_dir: &Path) -> Result<Vec<PathBuf>> {
    let mut repositories: Vec<PathBuf> = Vec::new();
    // Create a WalkDir iterator that is breadth-first and returns files before directories. This
    // allows us to skip a directory once we reach a file.
    let mut iter = WalkDir::new(vendor_dir)
        // ignore any files at the top of the vendor directory, such as `modules.txt`
        .min_depth(2)
        .contents_first(true)
        .sort_by(
            |a, b| match (a.file_type().is_file(), b.file_type().is_file()) {
                (true, false) => Ordering::Less,
                (false, true) => Ordering::Greater,
                _ => a.file_name().cmp(b.file_name()),
            },
        )
        .into_iter();
    while let Some(entry) = iter.next() {
        let entry = entry?;
        if entry.file_type().is_file() {
            repositories.push(
                entry
                    .path()
                    .parent()
                    .with_context(|| {
                        format!(
                            "a file must have a parent, received '{}'",
                            entry.path().display()
                        )
                    })?
                    .strip_prefix(vendor_dir)?
                    .into(),
            );
            iter.skip_current_dir();
        }
    }
    Ok(repositories)
}

#[cfg(test)]
mod test {
    // See `testdata/clarifications_sample.toml` in the crate root to see the test data.

    // Clippy doesn't like our checksums.
    #![allow(clippy::unreadable_literal)]

    use super::*;
    use maplit::btreemap;

    fn load_clarifications() -> Clarifications {
        toml::from_str(include_str!("../testdata/clarifications_sample.toml"))
            .expect("Failed to parse testdata")
    }

    #[test]
    fn test_match_unversioned_clarification() {
        // Given a clarification with no version specified
        let clarifications = load_clarifications();

        // When we match with no version,
        // Then a clarification is returned.
        assert_eq!(
            clarifications
                .get(
                    "singlepackage",
                    None,
                    btreemap! {
                        Path::new("LICENSE") => 0x00000000,
                        Path::new("NOTICE") => 0x00000000,
                    },
                )
                .unwrap(),
            Some(Clarified {
                expression: &spdx::Expression::parse("Apache-2.0").unwrap(),
                skip_files: &vec![],
            })
        );

        // When we match with some version,
        // Then a clarification is returned.
        assert_eq!(
            clarifications
                .get(
                    "singlepackage",
                    Some(&"5.0.0".parse().unwrap()),
                    btreemap! {
                        Path::new("LICENSE") => 0x00000000,
                        Path::new("NOTICE") => 0x00000000,
                    },
                )
                .unwrap(),
            Some(Clarified {
                expression: &spdx::Expression::parse("Apache-2.0").unwrap(),
                skip_files: &vec![],
            })
        );
    }

    #[test]
    fn test_match_versioned_package_versioned_clarification() {
        // Given a package and version specifier
        let clarifications = load_clarifications();

        // When the package and version match the requirements,
        // Then a clarification is returned.
        assert_eq!(
            clarifications
                .get(
                    "singleversioned",
                    Some(&"1.6.0".parse().unwrap()),
                    btreemap! {
                        Path::new("LICENSE") => 0x00000000,
                        Path::new("NOTICE") => 0x00000000,
                    },
                )
                .unwrap(),
            Some(Clarified {
                expression: &spdx::Expression::parse("MIT").unwrap(),
                skip_files: &vec![],
            })
        );

        // When the package and version do not match the requirements,
        // Then no clarification is returned.
        assert_eq!(
            clarifications
                .get(
                    "singleversioned",
                    Some(&"2.0.0".parse().unwrap()),
                    btreemap! {
                        Path::new("LICENSE") => 0x00000000,
                        Path::new("NOTICE") => 0x00000000,
                    },
                )
                .unwrap(),
            None
        );
    }

    #[test]
    fn test_match_unversioned_package_versioned_clarification() {
        // Given a package with no version specifier
        // When the clarified package has a version specified
        // Then no clarification is returned.
        let clarifications = load_clarifications();

        assert_eq!(
            clarifications
                .get(
                    "singleversioned",
                    None,
                    btreemap! {
                        Path::new("LICENSE") => 0x00000000,
                        Path::new("NOTICE") => 0x00000000,
                    },
                )
                .unwrap(),
            None
        );
    }

    #[test]
    fn test_multimatch() {
        // Given a package with a version specifier
        // When there are multiple packages with the same name
        // Then only the one with the matching version is returned.
        let clarifications = load_clarifications();

        assert_eq!(
            clarifications
                .get(
                    "multipackage",
                    Some(&"2.5.3".parse().unwrap()),
                    btreemap! {
                        Path::new("LICENSE") => 0x00000002,
                        Path::new("NOTICE") => 0x00000002,
                    },
                )
                .unwrap(),
            Some(Clarified {
                expression: &spdx::Expression::parse("Apache-2.0 OR BSD-3-Clause").unwrap(),
                skip_files: &vec![],
            })
        );
        assert_eq!(
            clarifications
                .get(
                    "multipackage",
                    Some(&"1.4.3".parse().unwrap()),
                    btreemap! {
                        Path::new("LICENSE") => 0x00000001,
                        Path::new("NOTICE") => 0x00000001,
                    },
                )
                .unwrap(),
            Some(Clarified {
                expression: &spdx::Expression::parse("Apache-2.0 OR MIT").unwrap(),
                skip_files: &vec![],
            })
        );
    }

    #[test]
    fn test_first_match() {
        // Given a package with a version specifier
        // When there are multiple packages with the same name and overlapping version specifiers
        // Then the first matching clarification is returned.
        let clarifications = load_clarifications();

        assert_eq!(
            clarifications
                .get(
                    "overlapping",
                    Some(&"1.3.1".parse().unwrap()),
                    btreemap! {
                        Path::new("LICENSE") => 0x00000000,
                        Path::new("NOTICE") => 0x00000000,
                    },
                )
                .unwrap(),
            Some(Clarified {
                expression: &spdx::Expression::parse("BSD-3-Clause").unwrap(),
                skip_files: &vec![],
            })
        );
        assert_eq!(
            clarifications
                .get(
                    "overlapping",
                    Some(&"1.1.2".parse().unwrap()),
                    btreemap! {
                        Path::new("LICENSE") => 0x00000000,
                        Path::new("NOTICE") => 0x00000000,
                    },
                )
                .unwrap(),
            Some(Clarified {
                expression: &spdx::Expression::parse("BSD-3-Clause AND Apache-2.0").unwrap(),
                skip_files: &vec![],
            })
        );
    }
}
