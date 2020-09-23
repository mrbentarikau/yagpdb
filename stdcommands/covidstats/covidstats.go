//inspired by https://disease.sh > https://github.com/disease-sh/api

package covidstats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/commands"
)

var (
	diseaseAPIHost = "https://disease.sh/v3/covid-19/"
	typeWorld      = "all"
	typeCountries  = "countries"
	typeContinents = "continents"
	typeStates     = "states"

	//These image links could just disappear, not trustworthy 100%.
	globeImage  = "http://pngimg.com/uploads/globe/globe_PNG63.png"
	footerImage = "https://upload-icon.s3.us-east-2.amazonaws.com/uploads/icons/png/2129370911599778130-512.png"

	africaImage       = "https://vemaps.com/uploads/img/af-c-05.png"
	asiaImage         = "https://vemaps.com/uploads/img/as-c-05.png"
	australiaImage    = "https://vemaps.com/uploads/img/oc-c-05.png"
	europeImage       = "https://vemaps.com/uploads/img/eu-c-05.png"
	northAmericaImage = "https://vemaps.com/uploads/img/na-c-05.png"
	southAmericaImage = "https://vemaps.com/uploads/img/sa-c-05.png"

	continentImages = map[string]string{
		"North America":     northAmericaImage,
		"Asia":              asiaImage,
		"South America":     southAmericaImage,
		"Europe":            europeImage,
		"Africa":            africaImage,
		"Australia/Oceania": australiaImage,
	}
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "CoronaStatistics",
	Aliases:     []string{"coronastats", "cstats", "cst"},
	Description: "Shows COVID-19 statistics sourcing Worldometer statistics. Location is country name or their ISO2/3 shorthand.\nIf nothing is added, shows World's total.\nListings are sorted by count of total cases not deaths.",
	RunInDM:     true,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Location", Type: dcmd.String},
	},
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "countries", Name: "Countries of the World"},
		&dcmd.ArgDef{Switch: "continents", Name: "The Continents of the World"},
		&dcmd.ArgDef{Switch: "states", Name: "A State name in USA"},
		&dcmd.ArgDef{Switch: "1d", Name: "Stats from yesterday"},
		&dcmd.ArgDef{Switch: "2d", Name: "Stats from the day before yesterday (does not apply to states)"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		var cStats coronaWorldWideStruct
		var cConts []coronaWorldWideStruct
		var queryType = typeWorld
		var whatDay = "current day"
		var yesterday = "false"
		var twoDaysAgo = "false"
		var where, queryURL string
		var pagination = false

		//to determine what will happen and what data gets shown
		if data.Switches["countries"].Value != nil && data.Switches["countries"].Value.(bool) {
			queryType = typeCountries
			pagination = true
		} else if data.Switches["continents"].Value != nil && data.Switches["continents"].Value.(bool) {
			queryType = typeContinents
			pagination = true
		} else if data.Switches["states"].Value != nil && data.Switches["states"].Value.(bool) {
			queryType = typeStates
			pagination = true
		}

		//day-back switches
		if data.Switches["1d"].Value != nil && data.Switches["1d"].Value.(bool) {
			whatDay = "yesterday"
			yesterday = "true"
		} else if data.Switches["2d"].Value != nil && data.Switches["2d"].Value.(bool) {
			whatDay = "day before yesterday"
			twoDaysAgo = "true"
			if queryType == typeStates {
				yesterday = "true"
				twoDaysAgo = "false"
			}
		}

		fmt.Println(len(data.Switches))
		//we make the final queryURL here
		queryURL = fmt.Sprintf("%s%s/%s", diseaseAPIHost, queryType, "?yesterday="+yesterday+"&twoDaysAgo="+twoDaysAgo+"&strict=true")
		if data.Args[0].Str() != "" {
			if queryType == typeWorld {
				queryType = typeCountries
			}
			where = data.Args[0].Str() //any time some non-switch text is entered, it's not paginated
			pagination = false
			queryURL = fmt.Sprintf("%s%s/%s", diseaseAPIHost, queryType, where+"?yesterday="+yesterday+"&twoDaysAgo="+twoDaysAgo+"&strict=true")
		}

		//let's get that API data
		body, err := getData(queryURL, where, queryType)
		if err != nil {
			return nil, err
		}

		//voodoo-hoodoo on detecting if JSON's array/object
		jsonDetector := bytes.TrimLeft(body, " \t\r\n")
		mapYes := len(jsonDetector) > 0 && jsonDetector[0] == '['
		mapNo := len(jsonDetector) > 0 && jsonDetector[0] == '{'
		if mapYes {
			queryErr := json.Unmarshal([]byte(body), &cConts)
			if queryErr != nil {
				return nil, queryErr
			}
		} else if mapNo {
			queryErr := json.Unmarshal([]byte(body), &cStats)
			if queryErr != nil {
				return nil, queryErr
			}
		}

		//let's render everything to slice
		cConts = append(cConts, cStats)

		//let's sort by total Covid-19 cases
		sort.SliceStable(cConts, func(i, j int) bool {
			return cConts[i].Cases > cConts[j].Cases
		})

		var embed = &discordgo.MessageEmbed{}
		embed = embedCreator(cConts, queryType, whatDay, 0)

		if pagination {
			_, err := paginatedmessages.CreatePaginatedMessage(
				data.GS.ID, data.CS.ID, 1, len(cConts)-1, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					embed = embedCreator(cConts, queryType, whatDay, page-1)
					return embed, nil
				})
			if err != nil {
				return "Something went wrong", nil
			}
		} else {
			return embed, nil
		}

		return nil, nil
	},
}

func getData(query, loc, qtype string) ([]byte, error) {
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "curlPAGST/7.65.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, commands.NewPublicError("Cannot fetch corona statistics data for the given location:** " + qtype + ": " + loc + "**")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func embedCreator(cConts []coronaWorldWideStruct, queryType, whatDay string, i int) *discordgo.MessageEmbed {

	embed := &discordgo.MessageEmbed{
		Description: fmt.Sprintf("showing corona statistics for " + whatDay + ":"),
		Color:       0x7b0e4e,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Population", Value: fmt.Sprintf("%d", cConts[i].Population), Inline: true},
			&discordgo.MessageEmbedField{Name: "Total Cases", Value: fmt.Sprintf("%d", cConts[i].Cases), Inline: true},
			&discordgo.MessageEmbedField{Name: "New Cases", Value: fmt.Sprintf("%d", cConts[i].TodayCases), Inline: true},
			&discordgo.MessageEmbedField{Name: "Total Deaths", Value: fmt.Sprintf("%d", cConts[i].Deaths), Inline: true},
			&discordgo.MessageEmbedField{Name: "New Deaths", Value: fmt.Sprintf("%d", cConts[i].TodayDeaths), Inline: true},
			&discordgo.MessageEmbedField{Name: "Recovered", Value: fmt.Sprintf("%d", cConts[i].Recovered), Inline: true},
			&discordgo.MessageEmbedField{Name: "Active", Value: fmt.Sprintf("%d", cConts[i].Active), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: "Stay safe, protect yourself and others!", IconURL: footerImage},
	}
	//this here is because USA states API does not give critical conditions and to continue proper layout
	if queryType != typeStates {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Critical", Value: fmt.Sprintf("%d", cConts[i].Critical), Inline: true})
	}
	embed.Fields = append(embed.Fields,
		&discordgo.MessageEmbedField{Name: "Cases/1M pop", Value: fmt.Sprintf("%.0f", cConts[i].CasesPerOneMillion), Inline: true},
		&discordgo.MessageEmbedField{Name: "Total Tests", Value: fmt.Sprintf("%.0f", cConts[i].Tests), Inline: true})
	switch queryType {
	case "all":
		embed.Title = fmt.Sprintf("Whole world")
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: globeImage}
	case "countries":
		embed.Title = fmt.Sprintf("%s (%s)", cConts[i].Country, cConts[i].CountryInfo.Iso2)
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: fmt.Sprintf("%s", cConts[i].CountryInfo.Flag)}
	case "continents":
		embed.Title = fmt.Sprintf("%s", cConts[i].Continent)
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: continentImages[cConts[i].Continent]}
	case "states":
		embed.Title = fmt.Sprintf("USA, %s", cConts[i].State)
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: "https://disease.sh/assets/img/flags/us.png"}
	}
	return embed
}
