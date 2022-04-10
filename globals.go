package main

import (
	"os"
	"log"
	"time"
	"github.com/bwmarrin/discordgo"
)

const (
	SELF_DESTRUCT_TIMER_DEFAULT = time.Duration(1 * time.Hour)
	SELF_DESTRUCT_TIMER_MIN = time.Duration(5 * time.Second)
	SELF_DESTRUCT_TIMER_MAX = time.Duration(24 * time.Hour)
	SELF_DESTRUCT_BACKUP_TIMER = time.Duration(10 * time.Minute)
	UT_MAX_TEAM_SIZE = 4
	UT_MAX_PLAYER_SIZE = 48
	MAX_AUTO_QUERY_NUMBER_PER_GUILD = 5
	EMBED_COLOUR_FULL = 0x0078D7
	EMBED_COLOUR_ACTIVE = 0x16C60C
	EMBED_COLOUR_ONLINE = 0xFFF100
	EMBED_COLOUR_OFFLINE = 0x383838
	SERVER_STATUS_FULL = "🟡"
	SERVER_STATUS_ACTIVE = "🟢"
	SERVER_STATUS_ONLINE = "🔵"
	SERVER_STATUS_OFFLINE = "⚫"
	DEFAULT_REACTION_EMOJI = "🖤"
	ASSERT_MISSING_VERSION = "MISSING VERSION INFORMATION. SEE BUILD SCRIPT OR REMOVE VERSION CHECKS FROM SOURCE."
	UT_ROLE_MENTIONS = true
)

var (
	gVersionLong = ASSERT_MISSING_VERSION
	gVersionShort = ASSERT_MISSING_VERSION
	gLogFile *os.File
	gLogger *log.Logger
	gAdmins []string
	gBot *discordgo.Session
	gExit = make(chan struct{})
	gSelfDestructMessages = make(chan SelfDestructMessage, 10)
	gUTNewAutoQueries = make(chan UTNewAutoQuery, 10)
	gUTDeleteAutoQueries = make(chan UTNewAutoQuery, 10)
	gUTAutoQueryLoopUpdates = make(chan UTAutoQueryLoopUpdate, 10)
	gUTQueryEvents = make(chan UTQueryEvent, 10)
	gUTAutoQueryLimitChecks = make(chan UTAutoQueryLimitCheck, 10)
	gMessageReactionAdds = make(chan discordgo.MessageReactionAdd, 10)
	gMessageReactionRemoves = make(chan discordgo.MessageReactionRemove, 10)
	gSlashCommands = []*discordgo.ApplicationCommand {
		{
			Name: "ut",
			Description: "Unreal Tournament GOTY slash commands.",
			Options: []*discordgo.ApplicationCommandOption {
				{
					Name: "add",
					Description: "How did you find this text, punk?!",
					Type: discordgo.ApplicationCommandOptionSubCommandGroup,
					Options: []*discordgo.ApplicationCommandOption {
						{
							Name: "server",
							Description: "Unreal Tournament GOTY live server updates.",
							Type: discordgo.ApplicationCommandOptionSubCommand,
							Options: []*discordgo.ApplicationCommandOption {
								{
									Name: "host",
									Description: "Example: 203.0.113.30:7777",
									Required: true,
									Type: discordgo.ApplicationCommandOptionString,
								},
								{
									Name: "name",
									Description: "Role and channel name. Example: Duel DE",
									Required: true,
									Type: discordgo.ApplicationCommandOptionString,
								},
								// {
								// 	Name: "mentions",
								// 	Description: "Assign a new role through reactions and mention when a player joins an empty server. Default: True",
								// 	Required: false,
								// 	Type: discordgo.ApplicationCommandOptionBoolean,
								// },
								// {
								// 	Name: "messages",
								// 	Description: "Send a message when a player joins or leaves the server. Default: True",
								// 	Required: false,
								// 	Type: discordgo.ApplicationCommandOptionBoolean,
								// },
								// {
								// 	Name: "timer",
								// 	Description: "Self-destruct timer for messages and mentions. Default: " + SELF_DESTRUCT_TIMER_DEFAULT.String(),
								// 	Required: false,
								// 	Type: discordgo.ApplicationCommandOptionString,
								// },
							},
						},
					},
				},
			},
		},
	}
)