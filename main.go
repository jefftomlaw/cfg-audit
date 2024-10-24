package main

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Claim struct {
	ChartNumber            string
	CaseNumber             string
	ClaimNo                string
	DateOfService          time.Time
	DateOfServiceString    string
	InsurancePaid          int64
	InsurancePaidString    string
	InsuranceName          string
	AdjustmentAmount       int64
	AdjustmentAmountString string
	Facility               string
	Sheet                  string
	Duplicate              bool
}

func readCsvFile(filePath string) [][]string {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Unable to read input file "+filePath, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal("Unable to parse file as CSV for "+filePath, err)
	}

	return records
}

func moneyStringToInt(str string) int64 {
	s := strings.ReplaceAll(str, ".", "")
	s = strings.ReplaceAll(s, "-", "")
	i, err := strconv.ParseInt(s, 10, 32)

	if err != nil {
		panic("failed to parse int")
	}

	return i
}

func parseDate(str string) time.Time {
	t, err := time.Parse("1/2/2006", str)
	if err != nil {
		panic("failed to parse time")
	}

	return t
}

func formatDateString(str string) string {
	t := parseDate(str)
	return t.Format("01/02/2006")
}

func parseRecords(records [][]string) (map[string][]Claim, error) {
	charts := make(map[string][]Claim)

	for _, value := range records {
		chartNo := value[0]

		//get the chart
		chartClaims := charts[chartNo]

		if chartClaims == nil {
			chartClaims = []Claim{}
		}

		claim := Claim{
			ChartNumber:            value[0],
			CaseNumber:             value[4],
			ClaimNo:                value[5],
			DateOfService:          parseDate(value[6]),
			DateOfServiceString:    formatDateString(value[6]),
			InsurancePaid:          moneyStringToInt(value[10]),
			InsurancePaidString:    strings.ReplaceAll(value[10], "-", ""),
			InsuranceName:          value[11],
			AdjustmentAmount:       moneyStringToInt(value[12]),
			AdjustmentAmountString: strings.ReplaceAll(value[12], "-", ""),
			Facility:               value[14],
			Sheet:                  value[15],
			Duplicate:              false,
		}

		updatedClaims := append(chartClaims, claim)
		sort.Slice(updatedClaims, func(i, j int) bool {
			return updatedClaims[i].DateOfService.Before(updatedClaims[j].DateOfService)
		})
		charts[chartNo] = updatedClaims
	}

	return charts, nil
}

func removeSingleClaimRecords(chartMap map[string][]Claim) map[string][]Claim {
	chartsWithMultipleClaims := make(map[string][]Claim)

	for key, value := range chartMap {
		if len(value) > 1 {
			chartsWithMultipleClaims[key] = value
		}
	}

	return chartsWithMultipleClaims
}

func removeSumZeroClaimRecords(chartMap map[string][]Claim) map[string][]Claim {
	charts := make(map[string][]Claim)

	//loop through the map
	for key, claims := range chartMap {
		//sum the total amount paid

		var sumPaid int64 = 0

		for _, claim := range claims {
			sumPaid += claim.InsurancePaid
		}

		if sumPaid > 0 {
			charts[key] = claims
		}
	}

	return charts
}

func markDuplicatePayments(chartMap map[string][]Claim) map[string][]Claim {
	charts := make(map[string][]Claim)

	//loop through the map
	for key, claims := range chartMap {
		//map by date of service
		dosMap := make(map[string][]Claim)

		//loop each claim for the chart
		for _, claim := range claims {

			//make sure an array exists
			if dosMap[claim.DateOfServiceString] == nil {
				dosMap[claim.DateOfServiceString] = []Claim{}
			}

			dosMap[claim.DateOfServiceString] = append(dosMap[claim.DateOfServiceString], claim)
		}

		//sort the claim array by
		for _, dosClaims := range dosMap {
			sort.Slice(dosClaims, func(i, j int) bool {
				return dosClaims[i].InsurancePaid > dosClaims[j].InsurancePaid
			})
		}

		//combine the date of service arrays
		allClaims := []Claim{}

		for _, dosClaims := range dosMap {
			//mark duplicates while combining
			for index := range dosClaims {
				if index > 0 {
					dosClaims[index].Duplicate = true
				}
			}

			allClaims = append(allClaims, dosClaims...)
		}
		charts[key] = allClaims
	}

	return charts
}

func main() {
	records := readCsvFile("./data.csv")
	chartMap, err := parseRecords(records)

	if err != nil {
		fmt.Println("Error parsing records: ", err)
	}

	err = createHtmlFile("step1_claims_by_chart.html", chartMap)

	if err != nil {
		panic(err)
	}

	multipleClaimChartMap := removeSingleClaimRecords(chartMap)

	err = createHtmlFile("step2_multiple_claim_charts.html", multipleClaimChartMap)

	if err != nil {
		panic(err)
	}

	noZeroSumClaims := removeSumZeroClaimRecords(multipleClaimChartMap)

	err = createHtmlFile("step3_nonzero_multiple_claim_charts.html", noZeroSumClaims)

	if err != nil {
		panic(err)
	}

	markedDups := markDuplicatePayments(noZeroSumClaims)

	err = createHtmlFile("step4_marked_duplicates.html", markedDups)

	if err != nil {
		panic(err)
	}

}

func createHtmlFile(filename string, data map[string][]Claim) error {

	//render multiple claims
	t, err := template.New(filename).Parse(tpl)

	if err != nil {
		return err
	}

	// Create the HTML file
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Execute the template and write to the file
	err = t.Execute(f, data)

	if err != nil {
		return err
	}

	return nil
}

// create the html file
const tpl string = `
<!DOCTYPE html>
<html>
<head>
    <title>CJG Reconciliation</title>
</head>
<body>
<style>
th, td {
  padding: 5px;
}

.chart-row td {
	font-weight: 800;
}
</style>
<h1>CJG Reconciliation</h1>
<table width="1000">
<tr>
	<th align="left">Service Date</th>
	<th align="left">Case #</th>
	<th align="left">Claim #</th>
	<th align="left">Insurance</th>
	<th align="right">Payment</th>
	<th align="right">Adjustment</th>
	<th align="left">Sheet</th>
</tr>
{{range $key, $value := .}}
	<tr class="chart-row">
		<td colspan="7">Chart No. {{$key}}</td>
	</tr>
	{{ range $value}}
	<tr {{if .Duplicate }}style="background:lightgrey;"{{end}}>		
		<td>{{.DateOfServiceString}}</td>
		<td>{{.CaseNumber}}</td>
		<td>{{.ClaimNo}}</td>	
		<td>{{.InsuranceName}}</td>	
		<td align="right">{{.InsurancePaidString}}</td>
		<td align="right">{{.AdjustmentAmountString}}</td>	
		<td>{{.Sheet}}</td>
	</tr>
	{{end}}
{{end}}
</table>
</body>
</html>
`
