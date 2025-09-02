package date_utils

import (
	"testing"
)

func TestParseDateTime01(t *testing.T) {
	pivotYear = 30
	layout := "06/01/02"
	var dateValues []string = []string{"79/01/15","04/01/15","49/01/15","44/03/03","51/11/15","34/02/03","53/09/02","47/07/26","46/11/04","53/09/10"}

	for _, value := range dateValues {
		tm, err := ParseDateTime(layout, value)
		if err != nil {
			t.Fatalf("error while parsing date %s, with layout %s: %v", value, layout, err)
		}
		if tm.Year() < 1900 || tm.Year() > 2030 {
			t.Fatalf("error invalid year %d for  date %s, with layout %s", tm.Year(), value, layout)
		}
	}
	// t.Error("Done")
}

func TestParseDateTime10(t *testing.T) {
	pivotYear = 30
	layout := "2006/01/02"
	var dateValues []string = []string{"1979/01/15","2004/01/15","1949/01/15","1944/03/03","1951/11/15","1934/02/03","1953/09/02","1947/07/26","1946/11/04","1953/09/10"}

	for _, value := range dateValues {
		tm, err := ParseDateTime(layout, value)
		if err != nil {
			t.Fatalf("error while parsing date %s, with layout %s: %v", value, layout, err)
		}
		if tm.Year() < 1900 || tm.Year() > 2030 {
			t.Fatalf("error invalid year %d for  date %s, with layout %s", tm.Year(), value, layout)
		}
	}
	// t.Error("Done")
}

func TestParseDateTime02(t *testing.T) {
	pivotYear = 30
	layout := "06/01/02"
	var dateValues []string = []string{"49/01/15","44/03/03","51/11/15","34/02/03","53/09/02","47/07/26","46/11/04","53/09/10",
	"44/05/02","43/12/15","58/10/24","59/01/29","49/06/10","71/07/06","58/04/16","37/04/16","63/09/23","60/01/08","48/03/14",
	"52/11/15","48/03/18","37/08/23","51/03/15","55/05/16","48/04/30","56/09/30","40/10/17","37/03/22","58/06/27","58/11/10",
	"40/03/10","55/08/08","51/05/13","51/12/04","46/11/17","50/09/25","52/12/13","59/09/05","64/01/08","40/07/28","75/09/26",
	"47/04/08","54/06/22","54/06/22","54/06/22","54/06/22","54/06/22","37/05/07","27/11/17","37/05/07","37/05/07","38/03/05",
	"27/11/17","38/03/05","38/03/05","37/05/07","37/05/07","38/03/05","27/11/17","38/03/05","37/05/07","39/11/08","39/11/08",
	"39/11/08","37/06/22","37/06/22","33/12/25","38/07/19","33/12/25","38/07/19","47/08/20","37/06/22","38/07/19","33/12/25",
	"38/07/19","38/11/16","38/11/16","30/01/30","43/06/08","43/06/08","43/06/08","43/06/08","37/05/17","59/03/25","59/03/25",
	"28/10/30","42/07/13","40/09/28","40/09/28","40/09/28","40/09/28","40/09/28","40/09/28","40/09/28","36/06/26","36/06/26",
	"44/08/25","56/05/08","35/09/03","34/10/31"}

	for _, value := range dateValues {
		tm, err := ParseDateTime(layout, value)
		if err != nil {
			t.Fatalf("error while parsing date %s, with layout %s: %v", value, layout, err)
		}
		if tm.Year() < 1900 || tm.Year() > 2030 {
			t.Fatalf("error invalid year %d for  date %s, with layout %s", tm.Year(), value, layout)
		}
	}
}
