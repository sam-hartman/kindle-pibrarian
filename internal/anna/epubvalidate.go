package anna

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// isEPUBZip reports whether a ZIP byte slice is actually an EPUB (not a CBZ,
// DOCX, ODT, or some other zip that merely starts with "PK"). Amazon rejects
// non-EPUB zips, so detectFileFormat must not label them "epub".
func isEPUBZip(data []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, f := range zr.File {
		if f.Name == "mimetype" {
			if b, err := readZipEntry(zr, "mimetype"); err == nil &&
				strings.TrimSpace(string(b)) == "application/epub+zip" {
				return true
			}
		}
		if f.Name == "META-INF/container.xml" {
			return true
		}
	}
	return false
}

// ValidateEPUB checks an EPUB for defects that make Amazon's Send-to-Kindle
// converter fail (E999) or that mean the file isn't really an EPUB. It is
// deliberately conservative — it only flags issues that are genuinely fatal, so
// it won't reject a book Amazon would otherwise accept. Returns nil if the EPUB
// is safe to send.
func ValidateEPUB(data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("not a valid EPUB (corrupt or truncated zip): %w", err)
	}

	var hasMimetype, hasContainer bool
	for _, f := range zr.File {
		switch f.Name {
		case "META-INF/encryption.xml":
			return errors.New("the file is DRM-protected (encryption.xml present); Amazon cannot convert it")
		case "mimetype":
			hasMimetype = true
		case "META-INF/container.xml":
			hasContainer = true
		}
	}
	if !hasMimetype {
		return errors.New("missing the required mimetype entry")
	}
	if !hasContainer {
		return errors.New("missing META-INF/container.xml")
	}

	containerBytes, err := readZipEntry(zr, "META-INF/container.xml")
	if err != nil {
		return fmt.Errorf("cannot read META-INF/container.xml: %w", err)
	}
	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(containerBytes, &container); err != nil {
		return fmt.Errorf("container.xml is malformed XML: %w", err)
	}
	if len(container.Rootfiles) == 0 || container.Rootfiles[0].FullPath == "" {
		return errors.New("container.xml does not reference an OPF package file")
	}

	opfPath := container.Rootfiles[0].FullPath
	opfBytes, err := readZipEntry(zr, opfPath)
	if err != nil {
		return fmt.Errorf("OPF package file %q is missing: %w", opfPath, err)
	}
	if err := wellFormedXML(opfBytes); err != nil {
		return fmt.Errorf("OPF package file %q is malformed XML: %w", opfPath, err)
	}

	return nil
}

func readZipEntry(zr *zip.Reader, name string) ([]byte, error) {
	f, err := zr.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// wellFormedXML returns an error if the bytes are not well-formed XML.
func wellFormedXML(b []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
