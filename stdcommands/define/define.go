package define

import (
	"fmt"
	"math/rand"

	"github.com/dpatrie/urbandictionary"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Define",
	Aliases:      []string{"df", "udict", "urban", "urbd"},
	Description:  "Look up an urban dictionary definition",
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Topic", Type: dcmd.String},
	},
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "p", Name: "Paginated output"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var paginatedView bool

		if data.Switches["p"].Value != nil && data.Switches["p"].Value.(bool) {
			paginatedView = true
		}

		qResp, err := urbandictionary.Query(data.Args[0].Str())
		if err != nil {
			return "Failed querying :(", err
		}

		if len(qResp.Results) < 1 {
			return "No result :(", nil
		}

		if paginatedView {
			_, err := paginatedmessages.CreatePaginatedMessage(
				data.GS.ID, data.CS.ID, 1, len(qResp.Results), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					i := page - 1

					paginatedEmbed := embedCreator(qResp.Results, i)
					return paginatedEmbed, nil
				})
			if err != nil {
				return "Something went wrong", nil
			}
		} else {
			result := qResp.Results[0]

			cmdResp := fmt.Sprintf("**%s**: %s\n*%s*\n*(<%s>)*", result.Word, result.Definition, result.Example, result.Permalink)
			if len(qResp.Results) > 1 {
				cmdResp += fmt.Sprintf(" *%d more results*", len(qResp.Results)-1)
			}
			return cmdResp, nil
		}

		return nil, nil
	},
}

func embedCreator(udResult []urbandictionary.Result, i int) *discordgo.MessageEmbed {
	definition := udResult[i].Definition
	if len(definition) > 2000 {
		definition = definition[0:2000] + "...\n\n(definition too long)"
	}
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: udResult[i].Word,
			URL:  udResult[i].Permalink,
		},
		Description: fmt.Sprintf("**Definition**: %s", definition),
		Color:       int(rand.Int63n(16777215)),
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Example:", Value: udResult[i].Example},
			&discordgo.MessageEmbedField{Name: "Author:", Value: udResult[i].Author},
			&discordgo.MessageEmbedField{Name: "Votes:", Value: fmt.Sprintf("Upvotes: %d\nDownvotes: %d", udResult[i].Upvote, udResult[i].Downvote)},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://upload.wikimedia.org/wikipedia/commons/thumb/8/82/UD_logo-01.svg/512px-UD_logo-01.svg.png",
		},
	}

	return embed
}
