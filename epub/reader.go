package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"path/filepath"

	"github.com/jpbougie/lcpserve/epub/opf"
	"github.com/jpbougie/lcpserve/xmlenc"

	"io"
	"sort"
	"strings"
)

const (
	CONTAINER_FILE   = "META-INF/container.xml"
	ENCRYPTION_FILE  = "META-INF/encryption.xml"
	ROOTFILE_ELEMENT = "rootfile"
)

type rootFile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

func findRootFiles(r io.Reader) ([]rootFile, error) {
	xd := xml.NewDecoder(r)
	var roots []rootFile
	for x, err := xd.Token(); x != nil && err == nil; x, err = xd.Token() {
		if err != nil {
			return nil, err
		}
		switch x.(type) {
		case xml.StartElement:
			start := x.(xml.StartElement)
			if start.Name.Local == ROOTFILE_ELEMENT {
				var file rootFile
				err = xd.DecodeElement(&file, &start)
				if err != nil {
					return nil, err
				}
				roots = append(roots, file)
			}
		}
	}

	return roots, nil
}

func (ep *Epub) addCleartextResources(names []string) {
	if ep.cleartextResources == nil {
		ep.cleartextResources = []string{}
	}

	for _, name := range names {
		ep.cleartextResources = append(ep.cleartextResources, name)
	}
}

func (ep *Epub) addCleartextResource(name string) {
	if ep.cleartextResources == nil {
		ep.cleartextResources = []string{}
	}

	ep.cleartextResources = append(ep.cleartextResources, name)
}

func Read(r *zip.Reader) (Epub, error) {
	var ep Epub
	container, err := findFileInZip(r, CONTAINER_FILE)
	fd, err := container.Open()
	if err != nil {
		return Epub{}, err
	}
	defer fd.Close()

	rootFiles, err := findRootFiles(fd)
	if err != nil {
		return Epub{}, err
	}

	packages := make([]opf.Package, len(rootFiles))
	for i, rootFile := range rootFiles {
		ep.addCleartextResource(rootFile.FullPath)
		file, err := findFileInZip(r, rootFile.FullPath)
		if err != nil {
			return Epub{}, err
		}
		packageFile, err := file.Open()
		if err != nil {
			return Epub{}, err
		}
		defer packageFile.Close()

		packages[i], err = opf.Parse(packageFile)
		packages[i].BasePath = filepath.Dir(rootFile.FullPath)
		addCleartextResources(&ep, packages[i])
		if err != nil {
			return Epub{}, err
		}
	}

	resources := make([]Resource, 0)

	for _, file := range r.File {
		if file.Name != "META-INF/encryption.xml" &&
			file.Name != "mimetype" {
			resources = append(resources, Resource{File: file, Output: new(bytes.Buffer)})
		}
		if strings.HasPrefix(file.Name, "META-INF") {
			ep.addCleartextResource(file.Name)
		}
	}

	var encryption *xmlenc.Manifest
	f, err := findFileInZip(r, ENCRYPTION_FILE)
	if err == nil {
		r, err := f.Open()
		if err != nil {
			return Epub{}, err
		}
		defer r.Close()
		m, err := xmlenc.Read(r)
		encryption = &m
	}

	ep.Package = packages
	ep.Resource = resources
	ep.Encryption = encryption
	sort.Strings(ep.cleartextResources)

	return ep, nil
}

func addCleartextResources(ep *Epub, p opf.Package) {
	// Look for cover, nav and NCX items
	for _, item := range p.Manifest.Items {
		if strings.Contains(item.Properties, "cover-image") ||
			strings.Contains(item.Properties, "nav") ||
			item.MediaType == "application/x-dtbncx+xml" {
			ep.addCleartextResource(filepath.Join(p.BasePath, item.Href))
		}
	}
}
