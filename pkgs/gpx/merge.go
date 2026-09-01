package gpx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"go.goblog.app/app/pkgs/bufferpool"
)

// MergeGpx merges multiple GPX files into a single GPX 1.1 document and
// writes it to w. The track, route and waypoint elements of the input files
// are copied byte for byte from the source, keeping all contained information.
func MergeGpx(w io.Writer, gpxFiles ...[]byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while merging GPX: %v", r)
		}
	}()
	if w == nil {
		return fmt.Errorf("no writer provided")
	}
	if len(gpxFiles) == 0 {
		return fmt.Errorf("no GPX files provided")
	}

	// The subtree buffer only needs to live until its contents are written
	// to w below, so it can be reused.
	subtrees := bufferpool.Get()
	var names []string
	var earliestTime *time.Time
	var xmlnsDecls []xml.Attr
	xmlnsSeen := make(map[string]bool)

	for _, gpxFile := range gpxFiles {
		name, t, elements, xmlns, err := parseGpxForMerge(gpxFile)
		if err != nil {
			return fmt.Errorf("error parsing GPX file: %w", err)
		}
		if name != "" {
			names = append(names, name)
		}
		if t != nil && (earliestTime == nil || t.Before(*earliestTime)) {
			earliestTime = t
		}
		for _, decl := range xmlns {
			if xmlnsSeen[decl.Name.Local] {
				continue
			}
			xmlnsSeen[decl.Name.Local] = true
			xmlnsDecls = append(xmlnsDecls, decl)
		}
		_, _ = subtrees.Write(elements)
	}

	header := bufferpool.Get()
	defer bufferpool.Put(header, subtrees)
	header.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	header.WriteString(`<gpx version="1.1" creator="GoBlog" xmlns="http://www.topografix.com/GPX/1/1"`)
	for _, decl := range xmlnsDecls {
		header.WriteString(` xmlns:`)
		header.WriteString(decl.Name.Local)
		header.WriteString(`="`)
		_ = xml.EscapeText(header, []byte(decl.Value))
		header.WriteString(`"`)
	}
	header.WriteString(`>`)
	if len(names) > 0 || earliestTime != nil {
		header.WriteString("<metadata>")
		if len(names) > 0 {
			header.WriteString("<name>")
			_ = xml.EscapeText(header, []byte(strings.Join(names, ", ")))
			header.WriteString("</name>")
		}
		if earliestTime != nil {
			header.WriteString("<time>")
			header.WriteString(earliestTime.UTC().Format(time.RFC3339))
			header.WriteString("</time>")
		}
		header.WriteString("</metadata>")
	}

	if _, err := w.Write(header.Bytes()); err != nil {
		return err
	}
	if _, err := w.Write(subtrees.Bytes()); err != nil {
		return err
	}
	_, err = io.WriteString(w, "</gpx>")
	return err
}

type gpxMetadata struct {
	Name string `xml:"name"`
	Time string `xml:"time"`
}

// parseGpxForMerge extracts the top-level name, time, track, route and
// waypoint elements from a GPX file by copying their raw byte ranges from the
// source, so the elements keep all their original content and formatting.
func parseGpxForMerge(src []byte) (name string, t *time.Time, elements []byte, xmlns []xml.Attr, err error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	var buf bytes.Buffer
	var rootSeen bool
	var depth int
	var captureStart int64
	var captureKind string
	for {
		tokenStart := dec.InputOffset()
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", nil, nil, nil, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if !rootSeen {
				if tok.Name.Local != "gpx" {
					return "", nil, nil, nil, errNotAGpxFile
				}
				rootSeen = true
				xmlns = prefixedXmlnsDecls(tok.Attr)
			} else if depth == 2 {
				switch tok.Name.Local {
				case "metadata", "name", "time", "trk", "rte", "wpt":
					captureStart, captureKind = tokenStart, tok.Name.Local
				}
			}
		case xml.EndElement:
			if depth == 2 && captureKind != "" {
				element := src[captureStart:dec.InputOffset()]
				switch captureKind {
				case "metadata":
					var md gpxMetadata
					if err := xml.Unmarshal(element, &md); err != nil {
						return "", nil, nil, nil, err
					}
					if name == "" && md.Name != "" {
						name = md.Name
					}
					if t == nil && md.Time != "" {
						if parsed, ok := parseGPXTimeBytes([]byte(md.Time)); ok {
							t = &parsed
						}
					}
				case "name":
					if name == "" {
						name = textContent(element)
					}
				case "time":
					if t == nil {
						if parsed, ok := parseGPXTimeBytes([]byte(textContent(element))); ok {
							t = &parsed
						}
					}
				default:
					buf.Write(element)
				}
				captureKind = ""
			}
			depth--
		}
	}
	if !rootSeen {
		return "", nil, nil, nil, errNotAGpxFile
	}
	return name, t, buf.Bytes(), xmlns, nil
}

func prefixedXmlnsDecls(attrs []xml.Attr) []xml.Attr {
	var decls []xml.Attr
	for _, a := range attrs {
		if a.Name.Space == "xmlns" && a.Name.Local != "" {
			decls = append(decls, a)
		}
	}
	return decls
}

func textContent(sub []byte) string {
	var text struct {
		Value string `xml:",chardata"`
	}
	_ = xml.Unmarshal(sub, &text)
	return text.Value
}
