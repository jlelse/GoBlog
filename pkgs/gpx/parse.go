package gpx

import (
	"bytes"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/HBTGmbH/gosaxml"
)

// Constants replicating the geodesy helpers used by the original gpxgo-based implementation.
const (
	gpxOneDegree         = 1000.0 * 10000.8 / 90.0
	gpxEarthRadius       = 6371 * 1000
	gpxStoppedSpeedLimit = 1.0 // km/h, below which a point pair counts as stopped
)

// maxGPXDepth limits the element nesting accepted while parsing. Real GPX
// documents nest at most around 8 levels, while the XML decoder panics on
// deeper nesting (its internal state arrays only hold 32 entries), so
// anything beyond this limit is rejected with an error instead.
const maxGPXDepth = 20

// parsePoint is a parsed GPX point with only the fields needed by the track feature.
type parsePoint struct {
	lat, lon float64
	ele      float64 // math.NaN() when absent
	t        time.Time
}

// parseSpeedDistance pairs a point-to-point speed (m/s) with its distance (m).
type parseSpeedDistance struct {
	speed    float64
	distance float64
}

// parseSegment accumulates the track points and statistics of a single <trkseg>.
type parseSegment struct {
	points                          []Point
	prev                            parsePoint
	hasPrev                         bool
	movingTime, stoppedTime         float64
	movingDistance, stoppedDistance float64
	speeds                          []parseSpeedDistance
	elevations                      []float64
}

// parseRoute accumulates the display points of a single <rte>.
type parseRoute struct {
	points []Point
}

func newParseSegment(capacity int) *parseSegment {
	return &parseSegment{
		points:     make([]Point, 0, capacity),
		speeds:     make([]parseSpeedDistance, 0, capacity),
		elevations: make([]float64, 0, capacity),
	}
}

// addPoint folds a parsed point into the segment: display path plus running statistics.
func (s *parseSegment) addPoint(p *parsePoint) {
	if s.hasPrev {
		dist := gpxDistance3D(p, &s.prev)
		seconds := p.t.Sub(s.prev.t).Seconds()
		var speedKmh float64
		if seconds > 0 {
			//nolint:staticcheck // keep math.Pow to preserve floating-point results
			speedKmh = (dist / 1000.0) / (seconds / math.Pow(60, 2))
		}
		if speedKmh <= gpxStoppedSpeedLimit {
			s.stoppedTime += seconds
			s.stoppedDistance += dist
		} else {
			s.movingTime += seconds
			s.movingDistance += dist
			s.speeds = append(s.speeds, parseSpeedDistance{speed: dist / seconds, distance: dist})
		}
	}
	s.elevations = append(s.elevations, p.ele)
	s.points = append(s.points, Point{Trunc(p.lat), Trunc(p.lon)})
	s.prev = *p
	s.hasPrev = true
}

// movingData returns the combined movement statistics of the segment.
func (s *parseSegment) movingData() MovingData {
	return MovingData{
		MovingTime:      s.movingTime,
		StoppedTime:     s.stoppedTime,
		MovingDistance:  s.movingDistance,
		StoppedDistance: s.stoppedDistance,
		MaxSpeed:        gpxCalcMaxSpeed(s.speeds),
	}
}

// uphillDownhill computes the elevation gain and loss of the segment.
func (s *parseSegment) uphillDownhill() UphillDownhill {
	elevsLen := len(s.elevations)
	if elevsLen == 0 {
		return UphillDownhill{}
	}
	smooth := make([]float64, elevsLen)
	for i, elev := range s.elevations {
		curr := elev
		if 0 < i && i < elevsLen-1 {
			prev, next := s.elevations[i-1], s.elevations[i+1]
			if !math.IsNaN(prev) && !math.IsNaN(next) && !math.IsNaN(elev) {
				curr = prev*0.3 + elev*0.4 + next*0.3
			}
		}
		smooth[i] = curr
	}
	var uphill, downhill float64
	for i := 1; i < len(smooth); i++ {
		if !math.IsNaN(smooth[i]) && !math.IsNaN(smooth[i-1]) {
			if d := smooth[i] - smooth[i-1]; d > 0 {
				uphill += d
			} else {
				downhill -= d
			}
		}
	}
	return UphillDownhill{Uphill: uphill, Downhill: downhill}
}

// gpxDistance3D mirrors gpxgo's distance calculation between two points.
func gpxDistance3D(a, b *parsePoint) float64 {
	absLat := math.Abs(a.lat - b.lat)
	absLon := math.Abs(a.lon - b.lon)
	if absLat > 0.2 || absLon > 0.2 {
		return gpxHaversineDistance(a.lat, a.lon, b.lat, b.lon)
	}
	coef := math.Cos(gpxToRad(a.lat))
	x := a.lat - b.lat
	y := (a.lon - b.lon) * coef
	distance2d := math.Sqrt(x*x+y*y) * gpxOneDegree
	if a.ele == b.ele {
		return distance2d
	}
	eleDiff := 0.0
	if !math.IsNaN(a.ele) && !math.IsNaN(b.ele) {
		eleDiff = a.ele - b.ele
	}
	//nolint:staticcheck // keep math.Pow to preserve floating-point results
	return math.Sqrt(math.Pow(distance2d, 2) + math.Pow(eleDiff, 2))
}

func gpxHaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := gpxToRad(lat1 - lat2)
	dLon := gpxToRad(lon1 - lon2)
	thisLat1 := gpxToRad(lat1)
	thisLat2 := gpxToRad(lat2)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(thisLat1)*math.Cos(thisLat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return gpxEarthRadius * c
}

func gpxToRad(x float64) float64 {
	return x / 180.0 * math.Pi
}

// gpxCalcMaxSpeed mirrors gpxgo's maximum speed calculation (95th percentile of filtered speeds).
func gpxCalcMaxSpeed(speedsDistances []parseSpeedDistance) float64 {
	if len(speedsDistances) < 3 {
		return 0
	}
	var sumDists float64
	for _, d := range speedsDistances {
		sumDists += d.distance
	}
	avgDist := sumDists / float64(len(speedsDistances))
	var variance float64
	for i := range speedsDistances {
		//nolint:staticcheck // keep math.Pow to preserve floating-point results
		variance += math.Pow(speedsDistances[i].distance-avgDist, 2)
	}
	stdDeviation := math.Sqrt(variance)
	filtered := make([]parseSpeedDistance, 0, len(speedsDistances))
	for i := range speedsDistances {
		if math.Abs(speedsDistances[i].distance-avgDist) <= stdDeviation*1.5 {
			filtered = append(filtered, speedsDistances[i])
		}
	}
	speeds := make([]float64, len(filtered))
	for i, sd := range filtered {
		speeds[i] = sd.speed
	}
	sort.Float64s(speeds)
	if len(speeds) == 0 {
		return 0
	}
	maxIdx := int(float64(len(speeds)) * 0.95)
	if maxIdx >= len(speeds) {
		maxIdx = len(speeds) - 1
	}
	if maxIdx < 0 {
		maxIdx = 0
	}
	return speeds[maxIdx]
}

// Ordered with the most common formats first. Fractional seconds after the
// seconds field are accepted by time.Parse even when the layout does not
// mention them, so a single layout covers timestamps with and without
// milliseconds; they are dropped via Truncate to match gpxgo, which strips
// them from the string before parsing.
var gpxTimeLayouts = []string{
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05Z",
	"2006-01-02 15:04:05",
}

// Element IDs for the token stack: the stack stores these small ints instead
// of strings so that element name bookkeeping allocates nothing per element.
const (
	gpxElID = iota + 1
	trksegElID
	rteElID
	trkptElID
	rteptElID
	wptElID
	eleElID
	timeElID
	nameElID
	metadataElID
)

var (
	gpxBytes      = []byte("gpx")
	trksegBytes   = []byte("trkseg")
	rteBytes      = []byte("rte")
	trkptBytes    = []byte("trkpt")
	rteptBytes    = []byte("rtept")
	wptBytes      = []byte("wpt")
	eleBytes      = []byte("ele")
	timeBytes     = []byte("time")
	nameBytes     = []byte("name")
	metadataBytes = []byte("metadata")
	latBytes      = []byte("lat")
	lonBytes      = []byte("lon")
	versionBytes  = []byte("version")
	ampBytes      = []byte("amp")
	ltBytes       = []byte("lt")
	gtBytes       = []byte("gt")
	quotBytes     = []byte("quot")
	aposBytes     = []byte("apos")
)

func gosaxmlElementID(local []byte) byte {
	switch {
	case bytes.Equal(local, gpxBytes):
		return gpxElID
	case bytes.Equal(local, trksegBytes):
		return trksegElID
	case bytes.Equal(local, rteBytes):
		return rteElID
	case bytes.Equal(local, trkptBytes):
		return trkptElID
	case bytes.Equal(local, rteptBytes):
		return rteptElID
	case bytes.Equal(local, wptBytes):
		return wptElID
	case bytes.Equal(local, eleBytes):
		return eleElID
	case bytes.Equal(local, timeBytes):
		return timeElID
	case bytes.Equal(local, nameBytes):
		return nameElID
	case bytes.Equal(local, metadataBytes):
		return metadataElID
	}
	return 0
}

func gosaxmlPointParent(id byte) byte {
	switch id {
	case trkptElID:
		return trksegElID
	case rteptElID:
		return rteElID
	case wptElID:
		return gpxElID
	}
	return 0
}

var (
	errGPXMalformedLat = errors.New("malformed lat")
	errGPXMalformedLon = errors.New("malformed lon")
	errGPXUnbalanced   = errors.New("unbalanced end element")
	errGPXTooDeep      = errors.New("too deeply nested")
	errNotAGpxFile     = errors.New("not a GPX file")
)

// parseFloatBytes parses b as a float64 without allocating.
func parseFloatBytes(b []byte) (float64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(unsafe.String(&b[0], len(b)), 64)
	return v, err == nil
}

// parseFloatTrimmed parses b as a float64 without allocating, ignoring
// surrounding space characters (matching strings.Trim(s, " ")).
func parseFloatTrimmed(b []byte) (float64, bool) {
	i, j := 0, len(b)
	for i < j && b[i] == ' ' {
		i++
	}
	for j > i && b[j-1] == ' ' {
		j--
	}
	if i == j {
		return 0, false
	}
	return parseFloatBytes(b[i:j])
}

// parseGPXTimeBytes mirrors gpxgo's timestamp parsing (fractional seconds
// are dropped) without allocating.
func parseGPXTimeBytes(timestr []byte) (time.Time, bool) {
	timestr = bytes.Trim(timestr, " \t\n\r")
	if len(timestr) == 0 {
		return time.Time{}, false
	}
	s := unsafe.String(&timestr[0], len(timestr))
	for _, layout := range gpxTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Truncate(time.Second), true
		}
	}
	return time.Time{}, false
}

// decodeEntities decodes the named and numeric character entity references
// that encoding/xml would have decoded in character data.
func decodeEntities(b []byte) string {
	i := bytes.IndexByte(b, '&')
	if i < 0 {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b))
	sb.Write(b[:i])
	b = b[i:]
	for len(b) > 0 {
		semi := bytes.IndexByte(b, ';')
		if semi <= 1 {
			sb.Write(b)
			break
		}
		ent := b[1:semi]
		rest := b[semi+1:]
		if decoded, ok := decodeEntity(ent); ok {
			sb.WriteString(decoded)
		} else {
			sb.WriteByte('&')
			sb.Write(ent)
			sb.WriteByte(';')
		}
		i := bytes.IndexByte(rest, '&')
		if i < 0 {
			sb.Write(rest)
			break
		}
		sb.Write(rest[:i])
		b = rest[i:]
	}
	return sb.String()
}

func decodeEntity(ent []byte) (string, bool) {
	switch {
	case bytes.Equal(ent, ampBytes):
		return "&", true
	case bytes.Equal(ent, ltBytes):
		return "<", true
	case bytes.Equal(ent, gtBytes):
		return ">", true
	case bytes.Equal(ent, quotBytes):
		return `"`, true
	case bytes.Equal(ent, aposBytes):
		return "'", true
	}
	if len(ent) > 2 && ent[0] == '#' {
		base := 10
		digits := ent[1:]
		if ent[1] == 'x' || ent[1] == 'X' {
			base = 16
			digits = ent[2:]
		}
		if len(digits) == 0 {
			return "", false
		}
		n := 0
		for _, c := range digits {
			var v int
			switch {
			case c >= '0' && c <= '9':
				v = int(c - '0')
			case base == 16 && c >= 'a' && c <= 'f':
				v = int(c-'a') + 10
			case base == 16 && c >= 'A' && c <= 'F':
				v = int(c-'A') + 10
			default:
				return "", false
			}
			if v >= base {
				return "", false
			}
			n = n*base + v
			if n > 0x10FFFF {
				return "", false
			}
		}
		if n > 0 && (n < 0xD800 || n > 0xDFFF) {
			return string(rune(n)), true
		}
		return "", false
	}
	return "", false
}

// parseGPX parses a GPX document into the compact track representation.
func parseGPX(gpxString string) (*Track, error) {
	dec, _ := decoderPool.Get().(gosaxml.Decoder)
	dec.Reset(strings.NewReader(gpxString))
	defer decoderPool.Put(dec)

	result := &Track{}
	emptyEle := math.NaN()

	// A <trkpt> element is never smaller than ~25 bytes, so the input size
	// gives a good estimate of the point count and avoids repeated slice
	// growth while parsing large tracks.
	estPoints := len(gpxString) / 100

	stack := make([]byte, 0, 32)
	version := "1.1"
	var seg *parseSegment
	segments := make([]*parseSegment, 0, 4)
	var route *parseRoute
	routes := make([]*parseRoute, 0, 4)
	var waypoints []Point
	var point parsePoint
	pointDepth := 0
	pointActive := false

	startPoint := func(t *gosaxml.Token) error {
		point = parsePoint{ele: emptyEle}
		pointDepth = len(stack)
		pointActive = true
		for _, attr := range t.Attr {
			switch {
			case bytes.Equal(attr.Name.Local, latBytes):
				v, ok := parseFloatBytes(attr.Value)
				if !ok {
					return errGPXMalformedLat
				}
				point.lat = v
			case bytes.Equal(attr.Name.Local, lonBytes):
				v, ok := parseFloatBytes(attr.Value)
				if !ok {
					return errGPXMalformedLon
				}
				point.lon = v
			}
		}
		return nil
	}

	var tok gosaxml.Token
	for {
		err := dec.NextToken(&tok)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch tok.Kind {
		case gosaxml.TokenTypeStartElement:
			if len(stack) >= maxGPXDepth {
				return nil, errGPXTooDeep
			}
			id := gosaxmlElementID(tok.Name.Local)
			stack = append(stack, id)
			switch id {
			case gpxElID:
				for _, attr := range tok.Attr {
					if bytes.Equal(attr.Name.Local, versionBytes) {
						version = string(attr.Value)
					}
				}
			case trksegElID:
				seg = newParseSegment(estPoints)
				segments = append(segments, seg)
			case rteElID:
				route = &parseRoute{points: make([]Point, 0, estPoints)}
				routes = append(routes, route)
			case trkptElID, rteptElID, wptElID:
				if parent := gosaxmlPointParent(id); len(stack) > 1 && stack[len(stack)-2] == parent {
					if err := startPoint(&tok); err != nil {
						return nil, err
					}
				}
			}
		case gosaxml.TokenTypeTextElement:
			if len(stack) == 0 {
				continue
			}
			switch stack[len(stack)-1] {
			case eleElID:
				if pointActive && len(stack) == pointDepth+1 {
					if v, ok := parseFloatTrimmed(tok.ByteData); ok {
						point.ele = v
					}
				}
			case timeElID:
				if pointActive && len(stack) == pointDepth+1 {
					if tm, ok := parseGPXTimeBytes(tok.ByteData); ok {
						point.t = tm
					}
				}
			case nameElID:
				if version == "1.0" {
					if len(stack) == 2 && stack[0] == gpxElID {
						result.Name += decodeEntities(tok.ByteData)
					}
				} else {
					if len(stack) == 3 && stack[0] == gpxElID && stack[1] == metadataElID {
						result.Name += decodeEntities(tok.ByteData)
					}
				}
			}
		case gosaxml.TokenTypeEndElement:
			id := gosaxmlElementID(tok.Name.Local)
			switch id {
			case trkptElID, rteptElID, wptElID:
				if pointActive && len(stack) == pointDepth {
					switch id {
					case trkptElID:
						if seg != nil {
							seg.addPoint(&point)
						}
					case rteptElID:
						if route != nil {
							route.points = append(route.points, Point{Trunc(point.lat), Trunc(point.lon)})
						}
					case wptElID:
						waypoints = append(waypoints, Point{Trunc(point.lat), Trunc(point.lon)})
					}
				}
				pointActive = false
			case trksegElID:
				seg = nil
			case rteElID:
				route = nil
			}
			// The XML decoder performs no validation of end element names, so
			// guard against unbalanced or mismatched end tags instead of
			// popping beyond or outside of the tracked elements.
			if len(stack) == 0 || stack[len(stack)-1] != id {
				return nil, errGPXUnbalanced
			}
			stack = stack[:len(stack)-1]
		}
	}

	result.Paths = make([][]Point, 0, len(segments)+len(routes))
	result.Points = make([]Point, 0, len(waypoints))
	var md *MovingData
	var ud *UphillDownhill
	for _, s := range segments {
		smd := s.movingData()
		sud := s.uphillDownhill()
		if md == nil {
			md = &smd
		} else {
			md.MaxSpeed = math.Max(md.MaxSpeed, smd.MaxSpeed)
			md.MovingDistance += smd.MovingDistance
			md.MovingTime += smd.MovingTime
			md.StoppedDistance += smd.StoppedDistance
			md.StoppedTime += smd.StoppedTime
		}
		if ud == nil {
			ud = &sud
		} else {
			ud.Uphill += sud.Uphill
			ud.Downhill += sud.Downhill
		}
		result.Paths = append(result.Paths, s.points)
	}
	for _, r := range routes {
		result.Paths = append(result.Paths, r.points)
	}
	result.Points = append(result.Points, waypoints...)
	result.MovingData = md
	result.UphillDownhill = ud

	return result, nil
}
