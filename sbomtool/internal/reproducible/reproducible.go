// Package reproducible provides SPDX and CycloneDX JSON encoders whose timestamp honors
// SOURCE_DATE_EPOCH and whose document ID is a UUIDv5 of the content.
package reproducible

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CycloneDX/cyclonedx-go"
	"github.com/anchore/syft/syft/format/common/cyclonedxhelpers"
	"github.com/anchore/syft/syft/format/common/spdxhelpers"
	"github.com/anchore/syft/syft/sbom"
	"github.com/google/uuid"
	"github.com/spdx/tools-golang/convert"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdx/v2/v2_2"
	"github.com/spdx/tools-golang/spdx/v2/v2_3"
)

// SourceDateEpochEnv is the environment variable holding the build timestamp.
const SourceDateEpochEnv = "SOURCE_DATE_EPOCH"

// documentIDNamespace is the UUIDv5 namespace for document IDs; changing it changes every ID.
var documentIDNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("sbomtool.bottlerocket.dev"))

// now is replaceable in tests.
var now = time.Now

// Timestamp returns the time to record as an SBOM's creation time: SOURCE_DATE_EPOCH if set,
// otherwise the current time. It is always in UTC and truncated to whole seconds.
func Timestamp() (time.Time, error) {
	raw, ok := os.LookupEnv(SourceDateEpochEnv)
	if !ok || raw == "" {
		return now().UTC().Truncate(time.Second), nil
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || secs < 0 {
		return time.Time{}, fmt.Errorf("invalid %s %q: must be a non-negative integer number of seconds", SourceDateEpochEnv, raw)
	}
	return time.Unix(secs, 0).UTC(), nil
}

// contentUUID derives a stable UUIDv5 from encoded document bytes.
func contentUUID(content []byte) uuid.UUID {
	return uuid.NewSHA1(documentIDNamespace, content)
}

// EncodeSPDX writes s as SPDX JSON of the given version ("2.2" or "2.3") with a reproducible
// creation time and document namespace.
func EncodeSPDX(w io.Writer, s sbom.SBOM, version string, pretty bool) error {
	ts, err := Timestamp()
	if err != nil {
		return err
	}

	doc := spdxhelpers.ToFormatModel(s)
	if doc == nil {
		return fmt.Errorf("unable to convert SBOM to SPDX document")
	}
	if doc.CreationInfo != nil {
		doc.CreationInfo.Created = ts.Format(time.RFC3339)
	}

	// Keep Syft's namespace prefix but swap its random UUID for a content-derived one.
	prefix := namespacePrefix(doc.DocumentNamespace)

	doc.DocumentNamespace = ""
	content, err := encodeSPDXDoc(doc, version, pretty)
	if err != nil {
		return err
	}
	doc.DocumentNamespace = prefix + contentUUID(content).String()

	out, err := encodeSPDXDoc(doc, version, pretty)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

// namespacePrefix strips the trailing random UUID from a Syft-generated SPDX namespace,
// returning the part to which a content-derived UUID should be appended.
func namespacePrefix(ns string) string {
	const uuidLen = 36
	if n := len(ns); n >= uuidLen {
		if _, err := uuid.Parse(ns[n-uuidLen:]); err == nil {
			return ns[:n-uuidLen]
		}
	}
	if ns == "" {
		return "https://anchore.com/syft/unknown-source-type/"
	}
	return strings.TrimSuffix(ns, "/") + "/"
}

func encodeSPDXDoc(latest *spdx.Document, version string, pretty bool) ([]byte, error) {
	var encodeDoc any
	var err error
	switch version {
	case "2.2":
		doc := v2_2.Document{}
		err = convert.Document(latest, &doc)
		encodeDoc = doc
	case "2.3":
		doc := v2_3.Document{}
		err = convert.Document(latest, &doc)
		encodeDoc = doc
	default:
		return nil, fmt.Errorf("unsupported SPDX version %q", version)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to convert SBOM to SPDX %s document: %w", version, err)
	}

	// Match Syft's spdxjson encoder exactly.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", " ")
	}
	if err := enc.Encode(encodeDoc); err != nil {
		return nil, fmt.Errorf("failed to encode SPDX document: %w", err)
	}
	return buf.Bytes(), nil
}

// EncodeCycloneDX writes s as CycloneDX JSON of the given spec version (e.g. "1.6") with a
// reproducible metadata timestamp and serial number.
func EncodeCycloneDX(w io.Writer, s sbom.SBOM, version string, pretty bool) error {
	specVersion, err := cyclonedxSpecVersion(version)
	if err != nil {
		return err
	}
	ts, err := Timestamp()
	if err != nil {
		return err
	}

	bom := cyclonedxhelpers.ToFormatModel(s)
	if bom.Metadata != nil {
		bom.Metadata.Timestamp = ts.Format(time.RFC3339)
	}

	bom.SerialNumber = ""
	content, err := encodeCycloneDXBOM(bom, specVersion, pretty)
	if err != nil {
		return err
	}
	bom.SerialNumber = contentUUID(content).URN()

	out, err := encodeCycloneDXBOM(bom, specVersion, pretty)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func encodeCycloneDXBOM(bom *cyclonedx.BOM, version cyclonedx.SpecVersion, pretty bool) ([]byte, error) {
	// Match Syft's cyclonedxjson encoder exactly.
	var buf bytes.Buffer
	enc := cyclonedx.NewBOMEncoder(&buf, cyclonedx.BOMFileFormatJSON)
	enc.SetPretty(pretty)
	enc.SetEscapeHTML(false)
	if err := enc.EncodeVersion(bom, version); err != nil {
		return nil, fmt.Errorf("failed to encode CycloneDX BOM: %w", err)
	}
	return buf.Bytes(), nil
}

func cyclonedxSpecVersion(version string) (cyclonedx.SpecVersion, error) {
	switch version {
	case "1.6", "":
		return cyclonedx.SpecVersion1_6, nil
	case "1.5":
		return cyclonedx.SpecVersion1_5, nil
	case "1.4":
		return cyclonedx.SpecVersion1_4, nil
	default:
		return 0, fmt.Errorf("unsupported CycloneDX version %q", version)
	}
}
