use anyhow::{ensure, Context, Result};
use serde::Deserialize;
use std::collections::HashSet;
use std::path::Path;
use walkdir::WalkDir;

#[derive(Debug, Deserialize, Default)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct SPDXOptions {
    /// A set of licenses to ignore from the SPDX license data.
    #[serde(default)]
    pub(crate) ignore_licenses: HashSet<String>,
}

impl SPDXOptions {
    /// Mogrifies SPDX licenses using a set of static rules.
    ///
    /// This function is implemented to work around quirks in the SPDX data that cause unusual behavior
    /// in askalono during license identification.
    pub(crate) fn preprocess_licenses(
        &self,
        input_path: impl AsRef<Path>,
        output_path: impl AsRef<Path>,
    ) -> Result<()> {
        let input_path = input_path.as_ref();
        let output_path = output_path.as_ref();
        ensure!(
            input_path.is_dir(),
            "License preprocessing input path is not a directory."
        );
        ensure!(
            output_path.is_dir(),
            "License preprocessing output path is not a directory."
        );
        ensure!(
            input_path.canonicalize()? != output_path.canonicalize()?,
            "License preprocessing must write to a new path"
        );

        let license_iter = WalkDir::new(input_path)
            .min_depth(1)
            .max_depth(1)
            .into_iter();

        for license_file in license_iter {
            let license_file = license_file?;

            if !license_file.file_type().is_file() {
                continue;
            }
            let license_path = license_file.path();
            if license_file
                .file_name()
                .to_string_lossy()
                .ends_with(".json")
                && self.should_include_license_file(license_path)?
            {
                let license_output_path = output_path.join(license_file.file_name());
                std::fs::copy(license_path, &license_output_path).with_context(|| {
                    format!(
                        "Failed to copy license from '{}' to '{}'",
                        license_path.display(),
                        license_output_path.display()
                    )
                })?;
            }
        }

        Ok(())
    }

    fn should_include_license_file(&self, filepath: impl AsRef<Path>) -> Result<bool> {
        let filepath = filepath.as_ref();
        let license_name = filepath
            .file_name()
            .with_context(|| {
                format!(
                    "License file '{}' seemingly has no filename",
                    filepath.display()
                )
            })?
            .to_str()
            .with_context(|| {
                format!(
                    "License filename '{}' not valid unicode",
                    filepath.display()
                )
            })?
            .strip_suffix(".json")
            .with_context(|| {
                format!(
                    "License filename '{}' does not end in '.json'",
                    filepath.display()
                )
            })?;

        Ok(!self.ignore_licenses.contains(license_name))
    }
}
