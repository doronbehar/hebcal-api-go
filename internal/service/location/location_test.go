package location

import (
	"net/url"
	"testing"
)

// TestLegacyTzToTzid pins the legacy numeric-timezone plus DST-rule mapping,
// including the reversed Etc/GMT sign convention.
func TestLegacyTzToTzid(t *testing.T) {
	cases := []struct{ tz, dst, want string }{
		{"2", "israel", "Asia/Jerusalem"},
		{"0", "none", "UTC"},
		{"-5", "none", "Etc/GMT-5"}, // reversed sign convention (UTC+5)
		{"3", "none", "Etc/GMT+3"},
		{"0", "eu", "Europe/London"},
		{"1", "eu", "Europe/Paris"},
		{"-5", "usa", "America/New_York"}, // tz*-1 => 5
		{"-8", "usa", "America/Los_Angeles"},
		{"99", "bogus", ""},
	}
	for _, tc := range cases {
		if got := legacyTzToTzid(tc.tz, tc.dst); got != tc.want {
			t.Errorf("legacyTzToTzid(%q,%q) = %q, want %q", tc.tz, tc.dst, got, tc.want)
		}
	}
}

// TestFromLatLongTzidLeniency pins the two url-decoding hacks for a
// latitude/longitude request's tzid: a raw UTC offset like "+03:00" (whose "+"
// url-decodes to " "), and "Etc/GMT+5" (whose "+" likewise url-decodes to " ",
// landing as "Etc/GMT 5").
func TestFromLatLongTzidLeniency(t *testing.T) {
	cases := []struct{ tzid, want string }{
		{"Etc/GMT+5", "Etc/GMT+5"},   // untouched, already valid
		{"Etc/GMT 5", "Etc/GMT+5"},   // "+" url-decoded to " "
		{"Etc/GMT 12", "Etc/GMT+12"}, // two-digit offset
		{" 03:00", "Etc/GMT+3"},      // "+03:00" url-decoded to " 03:00"
		{"-02:00", "Etc/GMT-2"},
	}
	for _, tc := range cases {
		q := url.Values{
			"latitude":  {"41.85"},
			"longitude": {"-87.65"},
			"tzid":      {tc.tzid},
		}
		loc, err := FromQuery(nil, q)
		if err != nil {
			t.Errorf("FromQuery(tzid=%q) unexpected error: %v", tc.tzid, err)
			continue
		}
		if loc == nil || loc.TimeZoneID != tc.want {
			t.Errorf("FromQuery(tzid=%q).TimeZoneID = %v, want %q", tc.tzid, loc, tc.want)
		}
	}
}

// TestTimezoneLookup pins the coordinate-to-timezone fallback for both
// latitude/longitude forms when no tzid (or legacy tz/dst) is given, and that
// a bad tzid or out-of-range coordinate is still rejected.
func TestTimezoneLookup(t *testing.T) {
	ok := []struct {
		query, tzid string
		il          bool
	}{
		{"latitude=40.71&longitude=-74.01", "America/New_York", false},
		{"latitude=31.77&longitude=35.21", "Asia/Jerusalem", true},
		{"latitude=0&longitude=-30", "Etc/GMT+2", false}, // open ocean
		{"ladeg=51&lamin=30&ladir=n&lodeg=0&lomin=7&lodir=w", "Europe/London", false},
		{"ladeg=33&lamin=52&ladir=s&lodeg=151&lomin=12&lodir=e", "Australia/Sydney", false},
	}
	for _, tc := range ok {
		q, _ := url.ParseQuery(tc.query)
		loc, err := FromQuery(nil, q)
		if err != nil {
			t.Errorf("%s: %v", tc.query, err)
			continue
		}
		if loc.TimeZoneID != tc.tzid || loc.IL != tc.il {
			t.Errorf("%s: tzid=%q il=%v, want %q %v", tc.query, loc.TimeZoneID, loc.IL, tc.tzid, tc.il)
		}
	}
	bad := []string{
		"latitude=40.71&longitude=-74.01&tzid=Nowhere/Fake",
		"latitude=91&longitude=0",
		"latitude=0&longitude=-181",
		"ladeg=91&lamin=0&ladir=n&lodeg=0&lomin=0&lodir=e",
		"ladeg=51&lamin=30&ladir=n&lodeg=0&lomin=7&lodir=w&tzid=Nowhere/Fake",
	}
	for _, query := range bad {
		q, _ := url.ParseQuery(query)
		if loc, err := FromQuery(nil, q); err == nil {
			t.Errorf("%s: want error, got %+v", query, loc)
		}
	}
}
