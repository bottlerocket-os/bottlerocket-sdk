#![deny(rust_2018_idioms)]
#![warn(clippy::pedantic)]
#![allow(clippy::redundant_closure_for_method_calls)]

use anyhow::{bail, Context, Result};
use argh::FromArgs;
use serde::Deserialize;
use std::collections::HashMap;
use std::fs::{self, File};
use std::io;
use std::path::{Path, PathBuf};
use url::Url;

const DEFAULT_LICENSES_CONF: &str = "Licenses.toml";

/// Stores arguments
#[derive(FromArgs, PartialEq, Debug)]
struct Args {
    /// configuration file with the licenses to be used
    #[argh(option, short = 'l', default = "DEFAULT_LICENSES_CONF.to_string()")]
    licenses_file: String,

    #[argh(subcommand)]
    subcommand: Subcommand,
}

/// Stores the subcommand to be executed
#[derive(FromArgs, Debug, PartialEq)]
#[argh(subcommand)]
enum Subcommand {
    SpdxId(SpdxIdArgs),
    Path(PathArgs),
    Fetch(FetchArgs),
}

/// Returns the spdx-id for the package
#[derive(FromArgs, Debug, PartialEq)]
#[argh(subcommand, name = "spdx-id")]
struct SpdxIdArgs {
    /// the package name used to look up for the licenses
    #[argh(positional)]
    package_name: String,
}

/// Creates a copy of the licenses files in the dest directory
#[derive(FromArgs, Debug, PartialEq)]
#[argh(subcommand, name = "fetch")]
struct FetchArgs {
    /// the destination folder for the licenses
    #[argh(positional)]
    destination: PathBuf,
}

/// Prints out a space-separated list of the paths to the licenses files
#[derive(FromArgs, Debug, PartialEq)]
#[argh(subcommand, name = "path")]
struct PathArgs {
    /// the package name used to look up for the licenses
    #[argh(positional)]
    package_name: String,
    /// the source folder where the licenses are
    #[argh(option, short = 'p')]
    prefix: Option<PathBuf>,
}

/// Holds the configurations for package's licenses
#[derive(Deserialize, Debug)]
struct PackageLicense {
    // The SPDX identifier for the package
    #[serde(rename(deserialize = "spdx-id"))]
    spdx_id: String,
    // The licenses that apply to the package
    licenses: Vec<License>,
}

/// Holds the configurations for a license
#[derive(Deserialize, Debug, Clone)]
struct License {
    // The path to the license to fetch
    #[serde(rename(deserialize = "license-url"))]
    license_url: Option<Url>,
    // The file name used to store the license
    path: String,
}

/// Prints the spdx id for the package
fn print_spdx_id<S>(packages_licenses: &HashMap<String, PackageLicense>, package: S) -> Result<()>
where
    S: AsRef<str>,
{
    let package = package.as_ref();
    let package_license = packages_licenses.get(package).context(format!(
        "Couldn't find configuration for package '{package}'"
    ))?;
    println!("{}", package_license.spdx_id);
    Ok(())
}

/// Prints a space separated list of paths
fn print_paths<S>(
    packages_licenses: &HashMap<String, PackageLicense>,
    package_name: S,
    prefix: Option<PathBuf>,
) -> Result<()>
where
    S: AsRef<str>,
{
    let package_name = package_name.as_ref();
    let package_license = packages_licenses.get(package_name).context(format!(
        "Couldn't find configuration for package '{package_name}'"
    ))?;
    println!(
        "{}",
        get_license_destinations(package_license, prefix).join(" ")
    );
    Ok(())
}

/// Fetches all the licenses for the passed map of package licenses
async fn fetch_all_licenses<P>(
    packages_licenses: &HashMap<String, PackageLicense>,
    dest: P,
) -> Result<()>
where
    P: AsRef<Path>,
{
    for package_license in packages_licenses.values() {
        fetch_licenses(package_license, &dest).await?;
    }

    Ok(())
}

/// Fetches the licenses in the `PackageLicense` object, and creates a copy of them in `dest`
async fn fetch_licenses<P>(package_license: &PackageLicense, dest: P) -> Result<()>
where
    P: AsRef<Path>,
{
    let dest = dest.as_ref();
    for license in &package_license.licenses {
        let path: PathBuf = dest.join(&license.path);

        if path.exists() {
            // Skip if the file already exists
            continue;
        }

        if let Some(license_url) = &license.license_url {
            match license_url.scheme() {
                "file" => {
                    fs::copy(license_url.path(), &path)
                        .context(format!("Failed to copy file from '{}'", license_url.path()))?;
                }
                "http" | "https" => {
                    let content = reqwest::get(license_url.clone())
                        .await
                        .context(format!("Failed to download file from '{license_url}'"))?
                        .text()
                        .await?;
                    let mut dest = File::create(&path).context(format!(
                        "Failed to create file '{}'",
                        path.display()
                    ))?;
                    io::copy(&mut content.as_bytes(), &mut dest).context(format!(
                        "Failed to copy content to '{}'",
                        path.display()
                    ))?;
                }
                _ => bail!(
                    "Invalid scheme for '{}', valid options are: ['file://', 'http://', 'https://']",
                    license_url
                ),
            };
        }
    }
    Ok(())
}

/// Returns a list of paths to the destination files for the licenses
fn get_license_destinations(
    package_license: &PackageLicense,
    dest: Option<PathBuf>,
) -> Vec<String> {
    let mut all_paths = Vec::new();
    let dest = match dest {
        None => Path::new("").into(),
        Some(dest) => dest,
    };

    for license in &package_license.licenses {
        all_paths.push(dest.join(&license.path).display().to_string());
    }

    all_paths
}

/// Parses a map of `PackageLicense` objects from an array of bytes
fn parse_licenses_file<P>(licenses_file: P) -> Result<HashMap<String, PackageLicense>>
where
    P: AsRef<Path>,
{
    let licenses_file = licenses_file.as_ref();
    Ok(toml::from_str(
        &fs::read_to_string(licenses_file)
            .context(format!("Failed to read file '{}'", licenses_file.display()))?,
    )?)
}

#[tokio::main]
async fn main() -> Result<()> {
    let args: Args = argh::from_env();
    let packages_licenses = parse_licenses_file(&args.licenses_file)?;

    match args.subcommand {
        Subcommand::SpdxId(spdxid_args) => {
            print_spdx_id(&packages_licenses, spdxid_args.package_name)?;
        }
        Subcommand::Path(path_args) => {
            print_paths(&packages_licenses, path_args.package_name, path_args.prefix)?;
        }
        Subcommand::Fetch(fetch_args) => {
            fetch_all_licenses(&packages_licenses, fetch_args.destination).await?;
        }
    }

    Ok(())
}

#[cfg(test)]
mod test_packages_licenses {
    use super::{get_license_destinations, parse_licenses_file};
    use anyhow::Result;
    use std::io;
    static TEST_PACKAGES_LICENSES: &str = include_str!("../tests/data/test-packages-licenses.toml");

    #[test]
    fn test_parse_toml_file() -> Result<()> {
        let mut tmplicense = tempfile::NamedTempFile::new()?;
        io::copy(&mut TEST_PACKAGES_LICENSES.as_bytes(), &mut tmplicense)?;
        assert!(parse_licenses_file(tmplicense).is_ok());
        Ok(())
    }

    #[test]
    fn test_use_path() -> Result<()> {
        let mut tmplicense = tempfile::NamedTempFile::new()?;
        io::copy(&mut TEST_PACKAGES_LICENSES.as_bytes(), &mut tmplicense)?;
        let packages_licences = parse_licenses_file(tmplicense)?;
        let package_license = packages_licences.get("the-package").unwrap();
        // Original file name is `license.txt`
        assert!(
            get_license_destinations(package_license, Some("./dest".into()))
                == vec!["./dest/license-path.txt"]
        );
        Ok(())
    }
}

#[cfg(test)]
mod test_fetch_license {
    use super::{fetch_licenses, License, PackageLicense};
    use anyhow::Result;
    use httptest::{matchers::request, responders::status_code, Expectation, Server};
    use std::fs;
    use url::Url;

    #[tokio::test]
    async fn test_fetch_license_from_file() -> Result<()> {
        let tmpdir = tempfile::tempdir()?;
        let tmplicense = tempfile::NamedTempFile::new()?;
        let package_license = PackageLicense {
            spdx_id: "spdx-id".to_string(),
            licenses: vec![License {
                license_url: Some(Url::parse(&format!(
                    "file://{}",
                    tmplicense.path().display()
                ))?),
                path: String::from("license-file.txt"),
            }],
        };
        fetch_licenses(&package_license, &tmpdir).await?;
        assert!(tmpdir
            .path()
            .join(String::from("license-file.txt"))
            .exists());
        Ok(())
    }

    #[tokio::test]
    async fn test_fetch_license_from_http() -> Result<()> {
        let tmpdir = tempfile::tempdir()?;
        let server = Server::run();
        let license_body = "A cool body for the license";

        server.expect(
            Expectation::matching(request::method_path("GET", "/license.txt"))
                .respond_with(status_code(200).body(license_body)),
        );

        let url = server.url("/license.txt");
        let package_license = PackageLicense {
            spdx_id: "spdx-id".to_string(),
            licenses: vec![License {
                license_url: Some(Url::parse(&url.to_string())?),
                path: String::from("license-file.txt"),
            }],
        };
        let path = tmpdir.path().join(String::from("license-file.txt"));
        fetch_licenses(&package_license, &tmpdir).await?;
        assert!(path.exists());
        let content = fs::read(path)?;
        assert!(content == license_body.as_bytes());
        Ok(())
    }
}
