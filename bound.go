package geo

import (
	"fmt"
	"math"
	"strings"
)

// A Bound represents an enclosed "box" in the 2D Euclidean or Cartesian plane.
// It does not know anything about the anti-meridian.
type Bound struct {
	sw, ne *Point
}

// NewBound creates a new bound given the paramters.
func NewBound(west, east, south, north float64) *Bound {
	return &Bound{
		sw: &Point{math.Min(east, west), math.Min(north, south)},
		ne: &Point{math.Max(east, west), math.Max(north, south)},
	}
}

// NewBoundFromPoints creates a new bound given two opposite corners.
// These corners can be either sw/ne or se/nw.
func NewBoundFromPoints(corner, oppositeCorner *Point) *Bound {
	b := &Bound{
		sw: corner.Clone(),
		ne: corner.Clone(),
	}

	b.Extend(oppositeCorner)
	return b
}

// NewBoundAroundPoint creates a new bound given a center point,
// and a distance from the center point in meters
func NewBoundAroundPoint(center *Point, distance float64) *Bound {
	if distance < 0 {
		panic("invalid distance around center")
	}
	return boundAroundPoint(center, distance)
}

// NewBoundFromMapTile creates a bound given an online map tile index.
// Panics if x or y is out of range for zoom level.
func NewBoundFromMapTile(x, y, z uint64) *Bound {
	maxIndex := uint64(1) << z
	if x < 0 || y < 0 || x >= maxIndex || y >= maxIndex {
		panic("tile index out of range")
	}

	shift := 31 - z
	if z > 31 {
		shift = 0
	}

	lng1, lat1 := scalarMercatorInverse(x<<shift, y<<shift, 31)
	lng2, lat2 := scalarMercatorInverse((x+1)<<shift, (y+1)<<shift, 31)

	return &Bound{
		sw: &Point{math.Min(lng1, lng2), math.Min(lat1, lat2)},
		ne: &Point{math.Max(lng1, lng2), math.Max(lat1, lat2)},
	}
}

// NewBoundFromGeoHash creates a new bound for the region defined by the GeoHash.
func NewBoundFromGeoHash(hash string) *Bound {
	west, east, south, north := geoHash2ranges(hash)
	return NewBound(west, east, south, north)
}

// NewBoundFromGeoHashInt64 creates a new bound from the region defined by the GeoHesh.
// bits indicates the precision of the hash.
func NewBoundFromGeoHashInt64(hash int64, bits int) *Bound {
	west, east, south, north := geoHashInt2ranges(hash, bits)
	return NewBound(west, east, south, north)
}

func geoHash2ranges(hash string) (float64, float64, float64, float64) {
	latMin, latMax := -90.0, 90.0
	lngMin, lngMax := -180.0, 180.0
	even := true

	for _, r := range hash {
		// TODO: index step could probably be done better
		i := strings.Index("0123456789bcdefghjkmnpqrstuvwxyz", string(r))
		for j := 0x10; j != 0; j >>= 1 {
			if even {
				mid := (lngMin + lngMax) / 2.0
				if i&j == 0 {
					lngMax = mid
				} else {
					lngMin = mid
				}
			} else {
				mid := (latMin + latMax) / 2.0
				if i&j == 0 {
					latMax = mid
				} else {
					latMin = mid
				}
			}
			even = !even
		}
	}

	return lngMin, lngMax, latMin, latMax
}

func boundAroundPoint(center *Point, distance float64) *Bound {
	radDist := distance / EarthRadius
	radLat := deg2rad(center.Lat())
	radLon := deg2rad(center.Lng())
	minLat := radLat - radDist
	maxLat := radLat + radDist

	var minLon, maxLon float64
	if minLat > MinLatitude && maxLat < MaxLatitude {
		deltaLon := math.Asin(math.Sin(radDist) / math.Cos(radLat))
		minLon = radLon - deltaLon
		if minLon < MinLongitude {
			minLon += 2 * math.Pi
		}
		maxLon = radLon + deltaLon
		if maxLon > MaxLongitude {
			maxLon -= 2 * math.Pi
		}
	} else {
		minLat = math.Max(minLat, MinLatitude)
		maxLat = math.Min(maxLat, MaxLatitude)
		minLon = MinLongitude
		maxLon = MaxLongitude
	}
	return &Bound{
		sw: &Point{rad2deg(minLon), rad2deg(minLat)},
		ne: &Point{rad2deg(maxLon), rad2deg(maxLat)},
	}
}

func geoHashInt2ranges(hash int64, bits int) (float64, float64, float64, float64) {
	latMin, latMax := -90.0, 90.0
	lngMin, lngMax := -180.0, 180.0

	var i int64
	i = 1 << uint(bits)

	for i != 0 {
		i >>= 1

		mid := (lngMin + lngMax) / 2.0
		if hash&i == 0 {
			lngMax = mid
		} else {
			lngMin = mid
		}

		i >>= 1
		mid = (latMin + latMax) / 2.0
		if hash&i == 0 {
			latMax = mid
		} else {
			latMin = mid
		}
	}

	return lngMin, lngMax, latMin, latMax
}

// Extend grows the bound to include the new point.
func (b *Bound) Extend(point *Point) *Bound {

	// already included, no big deal
	if b.Contains(point) {
		return b
	}

	b.sw.SetX(math.Min(b.sw.X(), point.X()))
	b.ne.SetX(math.Max(b.ne.X(), point.X()))

	b.sw.SetY(math.Min(b.sw.Y(), point.Y()))
	b.ne.SetY(math.Max(b.ne.Y(), point.Y()))

	return b
}

// Union extends this bounds to contain the union of this and the given bounds.
func (b *Bound) Union(other *Bound) *Bound {
	b.Extend(other.SouthWest())
	b.Extend(other.NorthWest())
	b.Extend(other.SouthEast())
	b.Extend(other.NorthEast())

	return b
}

// Contains determines if the point is within the bound.
// Points on the boundary are considered within.
func (b *Bound) Contains(point *Point) bool {

	if point.Y() < b.sw.Y() || b.ne.Y() < point.Y() {
		return false
	}

	if point.X() < b.sw.X() || b.ne.X() < point.X() {
		return false
	}

	return true
}

// Intersects determines if two bounds intersect.
// Returns true if they are touching.
func (b *Bound) Intersects(bound *Bound) bool {
	if bound.Contains(b.sw) || bound.Contains(b.ne) ||
		bound.Contains(b.SouthEast()) || bound.Contains(b.NorthWest()) {
		return true
	}

	// now check the completely inside case, only one condition required
	if b.Contains(bound.sw) {
		return true
	}

	return false
}

// Center returns the center of the bound.
func (b *Bound) Center() *Point {
	p := &Point{}
	p.SetX((b.ne.X() + b.sw.X()) / 2.0)
	p.SetY((b.ne.Y() + b.sw.Y()) / 2.0)

	return p
}

// Pad expands the bound in all directions by the amount given. The amount must be
// in the units of the bounds. Technically one can pad with negative value,
// but no error checking is done.
func (b *Bound) Pad(amount float64) *Bound {
	b.sw.SetX(b.sw.X() - amount)
	b.sw.SetY(b.sw.Y() - amount)

	b.ne.SetX(b.ne.X() + amount)
	b.ne.SetY(b.ne.Y() + amount)

	return b
}

// GeoPad expands the bound in all directions by the given amount of meters.
// Only applies if the data is Lng/Lat degrees.
func (b *Bound) GeoPad(meters float64) *Bound {
	dy := meters / 111131.75
	dx := dy / math.Cos(deg2rad(b.ne.Lat()+b.sw.Lat())/2.0)

	b.sw.SetLng(b.sw.Lng() - dx)
	b.sw.SetLat(b.sw.Lat() - dy)

	b.ne.SetLng(b.ne.Lng() + dx)
	b.ne.SetLat(b.ne.Lat() + dy)

	return b
}

// Height returns just the difference in the point's Y/Latitude.
func (b *Bound) Height() float64 {
	return b.ne.Y() - b.sw.Y()
}

// Width returns just the difference in the point's X/Longitude.
func (b *Bound) Width() float64 {
	return b.ne.X() - b.sw.X()
}

// GeoHeight returns the approximate height in meters.
// Only applies if the data is Lng/Lat degrees.
func (b *Bound) GeoHeight() float64 {
	return 111131.75 * b.Height()
}

// GeoWidth returns the approximate width in meters.
// Only applies if the data is Lng/Lat degrees.
func (b *Bound) GeoWidth(haversine ...bool) float64 {
	c := b.Center()

	A := &Point{b.sw[0], c[1]}
	B := &Point{b.ne[0], c[1]}

	return A.GeoDistanceFrom(B, yesHaversine(haversine))
}

// SouthWest returns the lower left corner of the bound.
func (b *Bound) SouthWest() *Point { return b.sw.Clone() }

// NorthEast returns the upper right corner of the bound.
func (b *Bound) NorthEast() *Point { return b.ne.Clone() }

// SouthEast returns the lower right corner of the bound.
func (b *Bound) SouthEast() *Point {
	newP := &Point{}
	newP.SetLat(b.sw.Lat()).SetLng(b.ne.Lng())
	return newP
}

// NorthWest returns the upper left corner of the bound.
func (b *Bound) NorthWest() *Point {
	newP := &Point{}
	newP.SetLat(b.ne.Lat()).SetLng(b.sw.Lng())
	return newP
}

// Empty returns true if it contains zero area or if
// it's in some malformed negative state where the left point is larger than the right.
// This can be caused by Padding too much negative.
func (b *Bound) Empty() bool {
	return b.sw.X() >= b.ne.X() || b.sw.Y() >= b.ne.Y()
}

// Equals returns if two bounds are equal.
func (b *Bound) Equals(c *Bound) bool {
	if b.sw.Equals(c.sw) && b.ne.Equals(c.ne) {
		return true
	}

	return false
}

// Clone returns a copy of the bound.
func (b *Bound) Clone() *Bound {
	return NewBoundFromPoints(b.sw, b.ne)
}

// String returns the string respentation of the bound in the form,
// [[west, east], [south, north]]
func (b *Bound) String() string {
	return fmt.Sprintf("[[%f, %f], [%f, %f]]", b.sw.X(), b.ne.X(), b.sw.Y(), b.ne.Y())
}

// ToMysqlPolygon converts the bound into a polygon to be used in a MySQL spacial query.
func (b *Bound) ToMysqlPolygon() string {
	// west, south, west, north, east, north, east, south, west, south
	return fmt.Sprintf("POLYGON((%f %f, %f %f, %f %f, %f %f, %f %f))", b.sw[0], b.sw[1], b.sw[0], b.ne[1], b.ne[0], b.ne[1], b.ne[0], b.sw[1], b.sw[0], b.sw[1])
}

// ToMysqlIntersectsCondition returns a condition defining the intersection
// of the column and the bound. To be used in a MySQL query.
func (b *Bound) ToMysqlIntersectsCondition(column string) string {
	return fmt.Sprintf("INTERSECTS(%s, GEOMFROMTEXT('%s'))", column, b.ToMysqlPolygon())
}
