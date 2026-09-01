package gpx

import (
	"strings"
	"testing"
)

func TestParse_basic(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><metadata><name>My &amp; Track</name></metadata><trk><trkseg>
<trkpt lat="52.0" lon="13.0"><ele>100</ele><time>2021-01-01T10:00:00Z</time></trkpt>
<trkpt lat="52.01" lon="13.01"><ele>110</ele><time>2021-01-01T10:01:00Z</time></trkpt>
</trkseg></trk><wpt lat="1.5" lon="2.5"/></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "My & Track" {
		t.Fatalf("name = %q", res.Name)
	}
	if len(res.Paths) != 1 || len(res.Paths[0]) != 2 {
		t.Fatalf("paths = %+v", res.Paths)
	}
	if res.Paths[0][0] != (Point{5200000.0 / 100000, 1300000.0 / 100000}) {
		t.Fatalf("first point = %v", res.Paths[0][0])
	}
	if len(res.Points) != 1 || res.Points[0] != (Point{Trunc(1.5), Trunc(2.5)}) {
		t.Fatalf("waypoints = %+v", res.Points)
	}
	if res.MovingData == nil || res.UphillDownhill == nil {
		t.Fatal("expected stats")
	}
	if res.MovingData.MovingTime != 60 || res.UphillDownhill.Uphill <= 0 {
		t.Fatalf("md=%+v ud=%+v", res.MovingData, res.UphillDownhill)
	}
}

// Millisecond timestamps are dropped to whole seconds, matching gpxgo.
func TestParse_millisecondTimes(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><trk><trkseg>
<trkpt lat="52.0" lon="13.0"><ele>100</ele><time>2021-08-26T12:45:15.557Z</time></trkpt>
<trkpt lat="52.01" lon="13.01"><ele>110</ele><time>2021-08-26T12:46:06.661Z</time></trkpt>
</trkseg></trk></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if res.MovingData == nil || res.MovingData.MovingTime != 51 {
		t.Fatalf("md=%+v, want moving time 51", res.MovingData)
	}
}

func TestParse_gpx10(t *testing.T) {
	res, err := Parse(`<gpx version="1.0" creator="x" xmlns="http://www.topografix.com/GPX/1/0"><name>Old</name><rte><rtept lat="3" lon="4"/><rtept lat="3.5" lon="4.5"/></rte></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "Old" {
		t.Fatalf("name = %q", res.Name)
	}
	if len(res.Paths) != 1 || len(res.Paths[0]) != 2 {
		t.Fatalf("paths = %+v", res.Paths)
	}
}

func TestParse_empty(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><trk><trkseg></trkseg></trk></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 1 || len(res.Paths[0]) != 0 {
		t.Fatalf("paths = %+v", res.Paths)
	}
	if res.MovingData == nil || res.UphillDownhill == nil {
		t.Fatal("expected zero stats for empty segment")
	}
	if res.MovingData.MovingTime != 0 || res.MovingData.MaxSpeed != 0 || res.UphillDownhill.Uphill != 0 {
		t.Fatalf("md=%+v ud=%+v", res.MovingData, res.UphillDownhill)
	}
}

func TestParse_noTrack(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	// Paths and Points are always non-nil, so that JSON responses contain empty arrays instead of null.
	if res.Paths == nil || len(res.Paths) != 0 || res.Points == nil || len(res.Points) != 0 {
		t.Fatalf("expected empty non-nil paths and points, got %+v %+v", res.Paths, res.Points)
	}
}

func TestParse_emptyInput(t *testing.T) {
	res, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 0 || len(res.Points) != 0 || res.Name != "" {
		t.Fatalf("expected empty result, got %+v", res)
	}
}

func TestParse_noEleTime(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><trk><trkseg><trkpt lat="1" lon="2"/><trkpt lat="3" lon="4"/></trkseg></trk></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 1 || len(res.Paths[0]) != 2 {
		t.Fatalf("paths = %+v", res.Paths)
	}
}

func TestParse_malformedLat(t *testing.T) {
	if _, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><trk><trkseg><trkpt lat="abc" lon="2"/></trkseg></trk></gpx>`); err == nil {
		t.Fatal("expected error for malformed lat")
	}
}

func TestParse_nestedPointIgnored(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><trk><trkseg><trkpt lat="1" lon="2"><extensions><trkpt lat="9" lon="9"/></extensions></trkpt></trkseg></trk><gpx><wpt lat="7" lon="7"/></gpx></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) != 1 || len(res.Paths[0]) != 0 {
		t.Fatalf("paths = %+v", res.Paths)
	}
	if len(res.Points) != 1 || res.Points[0] != (Point{Trunc(7), Trunc(7)}) {
		t.Fatalf("waypoints = %+v", res.Points)
	}
}

func TestParse_cdataName(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><metadata><name><![CDATA[My & <Track>]]></name></metadata><trk><trkseg><trkpt lat="52.0" lon="13.0"/></trkseg></trk></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "My & <Track>" {
		t.Fatalf("name = %q", res.Name)
	}
}

func TestParse_cdataInIgnoredElements(t *testing.T) {
	res, err := Parse(`<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><metadata><name><![CDATA[Name]]></name></metadata><trk><trkseg>
<trkpt lat="52.0" lon="13.0"><ele>100</ele><desc><![CDATA[a < b & c > d]]></desc><cmt><![CDATA[]]></cmt></trkpt>
<trkpt lat="52.01" lon="13.01"><ele>110</ele><desc><![CDATA[another]]> trailing</desc></trkpt>
</trkseg></trk></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "Name" {
		t.Fatalf("name = %q", res.Name)
	}
	if len(res.Paths) != 1 || len(res.Paths[0]) != 2 {
		t.Fatalf("paths = %+v", res.Paths)
	}
}

func TestParse_cdataMultipleAndEmpty(t *testing.T) {
	res, err := Parse(`<gpx version="1.1"><metadata><name><![CDATA[A]]> &amp; <![CDATA[B]]></name></metadata></gpx>`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "A & B" {
		t.Fatalf("name = %q", res.Name)
	}
}

func TestParse_cdataUnterminated(t *testing.T) {
	if _, err := Parse(`<gpx version="1.1"><metadata><name><![CDATA[unterminated</name></metadata></gpx>`); err == nil {
		t.Fatal("expected error for unterminated CDATA section")
	}
}

func TestParse_unbalancedEnd(t *testing.T) {
	for _, s := range []string{
		`</gpx>`,
		`<gpx></gpx></gpx>`,
		`<gpx><metadata></gpx></metadata>`,
		`<gpx><trk><trkseg><trkpt lat="1" lon="2"></trkseg></trkpt></trk></gpx>`,
	} {
		if _, err := Parse(s); err == nil {
			t.Fatalf("expected error for unbalanced end tag: %q", s)
		}
	}
}

func TestParse_tooDeep(t *testing.T) {
	s := `<gpx version="1.1">` + strings.Repeat("<a>", 25) + strings.Repeat("</a>", 25) + `</gpx>`
	if _, err := Parse(s); err == nil {
		t.Fatal("expected error for too deeply nested GPX")
	}
}

func FuzzParse(f *testing.F) {
	f.Add(`<gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"><ele>3</ele><time>2021-01-01T10:00:00Z</time></trkpt></trkseg></trk></gpx>`)
	f.Add(`</gpx>`)
	f.Add(`<gpx><metadata></gpx></metadata>`)
	f.Add(`<gpx><![CDATA[unterminated`)
	f.Add(`<gpx>` + strings.Repeat("<a>", 25))
	f.Add(`<gpx><trkpt lat="1" lon="2"><desc><![CDATA[a < b]]></desc></trkpt></gpx>`)
	f.Add(``)
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s)
	})
}
