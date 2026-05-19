package humanreadable

import (
	"fmt"
	"testing"
)

// Here I present two ways to make unit tests. As you can see the Example
// functions work when the output is a string, especially if the string is
// combined from several assertions. The TestF functions are preferred since
// you can test the output more programmatically (and with types other than
// strings) and perform several assertions at once using for loops or loops
// over a test table (table-driven).

func ExampleIEC_kib01() {
	fmt.Println(IEC(50000))
	// Output: 48.8 KiB
}

func ExampleIEC_mib01() {
	fmt.Println(IEC(1500000))
	// Output: 1.4 MiB
}
func ExampleIEC_mib02() {
	fmt.Println(IEC(15555555))
	// Output: 14.8 MiB
}

func ExampleIEC_gib01() {
	fmt.Println(IEC(1900000000))
	// Output: 1.8 GiB
}
func ExampleIEC_tib01() {
	fmt.Println(IEC(20000000000000))
	// Output: 18.2 TiB
}
func ExampleIEC_pib01() {
	fmt.Println(IEC(2000000000000000))
	// Output: 1.8 PiB
}
func ExampleIEC_eib01() {
	fmt.Println(IEC(2000000000000000000))
	// Output: 1.7 EiB
}

func TestIEC(t *testing.T) {
	tables := []struct {
		x int64
		h string
	}{
		{1023, "1023 B"},
		{50000, "48.8 KiB"},
		{1500000, "1.4 MiB"},
		{15555555, "14.8 MiB"},
		{1900000000, "1.8 GiB"},
		{20000000000000, "18.2 TiB"},
		{2000000000000000, "1.8 PiB"},
		{2000000000000000000, "1.7 EiB"},
	}

	for _, table := range tables {
		output := IEC(table.x)
		if output != table.h {
			t.Errorf("IEC(%d) was incorrect, got: %s, want: %s", table.x, output, table.h)
		}
	}
}

func TestSI(t *testing.T) {
	tables := []struct {
		x int64
		h string
	}{
		{999, "999 B"},
		{1023, "1.0 kB"},
		{12350, "12.3 kB"},
		{12351, "12.4 kB"},
		{50000, "50.0 kB"},
		{1500000, "1.5 MB"},
		{15555555, "15.6 MB"},
		{1900000000, "1.9 GB"},
		{19999999999999, "20.0 TB"},
		{20000000000000, "20.0 TB"},
		{1999999999999999, "2.0 PB"},
		{2000000000000000, "2.0 PB"},
		{1999999999999999999, "2.0 EB"},
		{2000000000000000000, "2.0 EB"},
	}
	for _, table := range tables {
		output := SI(table.x)
		if output != table.h {
			t.Errorf("SI(%d) was incorrect, got: %s, want: %s", table.x, output, table.h)
		}
	}
}
