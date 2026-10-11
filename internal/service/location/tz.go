package location

import (
	"sync"

	tzf "github.com/ringsaturn/tzf/v2"
)

// tzFinder is built on first use: the embedded polygon data takes a few
// milliseconds to index, and most requests name a location that already
// carries its timezone.
var tzFinder = sync.OnceValue(func() tzf.F {
	f, err := tzf.NewDefaultFinder()
	if err != nil {
		return nil
	}
	return f
})

// TimezoneAt returns the IANA timezone containing the given coordinates, or ""
// when none can be determined. Open ocean resolves to a nautical Etc/GMT±N
// zone. The coordinates must already be range-checked.
func TimezoneAt(latitude, longitude float64) string {
	f := tzFinder()
	if f == nil {
		return ""
	}
	return f.GetTimezoneName(longitude, latitude)
}
