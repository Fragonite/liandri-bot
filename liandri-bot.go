package main

import (
	"time"
	"os"
	"os/signal"
	"syscall"
	"github.com/bwmarrin/discordgo"
	"strings"
	"net"
	"strconv"
	"encoding/csv"
)

func main() {
	// gLogger.Println(time.Now().UTC().Format(time.RFC3339))
	
	gBot.UpdateStatusComplex(discordgo.UpdateStatusData{Activities: []*discordgo.Activity{{Name: "Unreal Tournament", Type: discordgo.ActivityTypeWatching}}})
	
	var c = make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<- c
	close(gExit)
	gBot.Close()
	gLogger.Println("Shutting down.")
	time.Sleep(5 * time.Second)
}

func interactionCreateHandler(s *discordgo.Session, in *discordgo.InteractionCreate) {
	switch in.ApplicationCommandData().Name {
	case "ut":
		switch in.ApplicationCommandData().Options[0].Name {
		case "add":
			switch in.ApplicationCommandData().Options[0].Options[0].Name {
			case "server":
				var naq = UTNewAutoQuery{aq: UTAutoQuery{guildID: in.GuildID, timer: SELF_DESTRUCT_TIMER_DEFAULT}}
				// var aq = UTAutoQuery{guildID: in.GuildID, joinMessages: true, leaveMessages: true, timer: SELF_DESTRUCT_TIMER_DEFAULT}
				var err error
				var sHost, sPort string
				var iPort int
				// var mentions bool = true
				var dur time.Duration
				var errorMessages strings.Builder
				var name string
				var admin bool
				errorMessages.Grow(256)
				errorMessages.WriteString("```asciidoc\n")

				
				if in.Member == nil {
					gLogger.Println()
					errorMessages.WriteString("Error :: Member permissions could not be accessed.\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					return
				}

				for _, v := range gAdmins {
					if in.Member.User.ID == v {
						admin = true
						break
					}
				}
				
				if !(admin) && in.Member.Permissions & discordgo.PermissionManageChannels != discordgo.PermissionManageChannels {
					gLogger.Println()
					errorMessages.WriteString("Error :: Member does not have permission to manage channels.\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					return
				}
				
				for _, v := range in.ApplicationCommandData().Options[0].Options[0].Options {
					switch v.Name {
					case "host":
						sHost, sPort, err = net.SplitHostPort(v.StringValue())
						if err != nil {
							gLogger.Println(err)
							errorMessages.WriteString("Error :: Host could not be parsed.\n")
							break
						}
						
						ipHost := net.ParseIP(sHost)
						if ipHost == nil {
							gLogger.Println(sHost)
							errorMessages.WriteString("Error :: IP could not be parsed.\n")
							// break
						}
						
						iPort, err = strconv.Atoi(sPort)
						if err != nil {
							gLogger.Println(sPort)
							errorMessages.WriteString("Error :: Port could not be parsed.\n")
							// break
						}
						
					case "name":
						name = v.StringValue()
						
					case "mentions":
						// mentions = v.BoolValue()
						
					case "messages":
						naq.aq.joinMessages = v.BoolValue()
						naq.aq.leaveMessages = v.BoolValue()
						
					case "timer":
						dur, err = time.ParseDuration(v.StringValue())
						if err != nil {
							gLogger.Println(err)
							errorMessages.WriteString("Error :: Timer could not be parsed.\n")
							break
						}
						
						if dur < SELF_DESTRUCT_TIMER_MIN || dur > SELF_DESTRUCT_TIMER_MAX {
							gLogger.Println(dur)
							errorMessages.WriteString("Error :: Timer must be within range [" + SELF_DESTRUCT_TIMER_MIN.String() + ", " + SELF_DESTRUCT_TIMER_MAX.String() + "].")
							break
						}
						
						naq.aq.timer = dur
					}
				}
				
				if errorMessages.Len() > len("```asciidoc\n") {
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					return
				}
				
				var timer = time.NewTimer(2 * time.Second)
				var q0 = make(chan string)
				var q1 = make(chan string)
				var queryResult string
				
				go func(queryAddress string, ch chan string) {
					if result := utQueryServer(queryAddress); result != "" {
						ch <- result
					}
					time.Sleep(10 * time.Second)
					close(ch)
				}(net.JoinHostPort(sHost, strconv.Itoa(iPort)), q0)
				
				go func(queryAddress string, ch chan string) {
					if result := utQueryServer(queryAddress); result != "" {
						ch <- result
					}
					time.Sleep(10 * time.Second)
					close(ch)
				}(net.JoinHostPort(sHost, strconv.Itoa(iPort + 1)), q1)
				
				select {
				case queryResult = <- q0:
				case queryResult = <- q1:
					iPort++
				case <- timer.C:
					gLogger.Println()
					errorMessages.WriteString("Error :: Server did not respond (request timed out).\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					return
				}
				
				naq.qe = utNewQueryEvent(net.JoinHostPort(sHost, strconv.Itoa(iPort)), queryResult)
				
				if naq.qe.online == false {
					gLogger.Println(naq.qe)
					errorMessages.WriteString("Error :: Malformed server response.\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					return
				}
				
				// gLogger.Println("Limit Check Start")
				
				if !(admin) {
					var lc = UTAutoQueryLimitCheck{
						queryAddress: net.JoinHostPort(sHost, strconv.Itoa(iPort)),
						guildID: in.GuildID,
						err: make(chan error),
					}
					gUTAutoQueryLimitChecks <- lc
					err := <- lc.err
					close(lc.err)
					
					if err != nil {
						gLogger.Println(err)
						errorMessages.WriteString("Error :: " + err.Error() + "\n")
						errorMessages.WriteString("```")
						s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: errorMessages.String(),
								Flags: 1 << 6,
							},
						})
						return
					}
					
				}
				
				// gLogger.Println("Limit Check End")

				// if !(admin) {
				// 	//TODO: CHECK AGAINST ADMIN WHITELIST (DONE ACTUALLY) AND MAX NUMBER OF AUTO QUERY ENTRIES PER GUILD.
				// 	var duplicateCheck = UTAutoQueryDuplicateCheck{
				// 		queryAddress: net.JoinHostPort(sHost, strconv.Itoa(iPort)),
				// 		guildID: in.GuildID,
				// 		result: make(chan bool),
				// 	}
				// 	gUTAutoQueryDuplicateChecks <- duplicateCheck
				// 	var duplicate = <- duplicateCheck.result
				// 	close(duplicateCheck.result)
					
				// 	if duplicate {
				// 		gLogger.Println(duplicateCheck)
				// 		errorMessages.WriteString("Error :: An entry for this server already exists.\n")
				// 		errorMessages.WriteString("```")
				// 		s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
				// 			Type: discordgo.InteractionResponseChannelMessageWithSource,
				// 			Data: &discordgo.InteractionResponseData{
				// 				Content: errorMessages.String(),
				// 				Flags: 1 << 6,
				// 			},
				// 		})
				// 		return
				// 	}
				// }
				
				channel, err := s.GuildChannelCreateComplex(in.GuildID, discordgo.GuildChannelCreateData{
					Name: utGetServerStatus(&naq.qe) + name,
					Type: discordgo.ChannelTypeGuildText,
					PermissionOverwrites: []*discordgo.PermissionOverwrite{
						{
							ID: in.GuildID,//@everyone
							Type: discordgo.PermissionOverwriteTypeRole,
							Deny: discordgo.PermissionAddReactions | discordgo.PermissionSendMessages,
						},
					},
				})
				
				if err != nil {
					gLogger.Println(err)
					errorMessages.WriteString("Error :: Could not create channel.\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					return
				}
				
				role, err := s.GuildRoleCreate(in.GuildID)
				
				if err != nil {
					gLogger.Println(err)
					errorMessages.WriteString("Error :: Could not create role.\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					
					_, err = s.ChannelDelete(channel.ID)
					if err != nil {
						gLogger.Println(err)
					}
					
					return
				}
				
				role, err = s.GuildRoleEdit(in.GuildID, role.ID, name, role.Color, false, 0, true)
				if err != nil {
					gLogger.Println(err)
				}
				
				embed, err := utNewEmbed(naq.qe)
				if err != nil {
					gLogger.Println(err)
					errorMessages.WriteString("Error :: Could not create embed.\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					
					_, err = s.ChannelDelete(channel.ID)
					if err != nil {
						gLogger.Println(err)
					}
					
					err = s.GuildRoleDelete(in.GuildID, role.ID)
					if err != nil {
						gLogger.Println(err)
					}
					
					return
				}
				
				message, err := s.ChannelMessageSendEmbed(channel.ID, &embed)
				if err != nil {
					gLogger.Println(err)
					errorMessages.WriteString("Error :: Could not send embed.\n")
					errorMessages.WriteString("```")
					s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: errorMessages.String(),
							Flags: 1 << 6,
						},
					})
					
					_, err = s.ChannelDelete(channel.ID)
					if err != nil {
						gLogger.Println(err)
					}
					
					err = s.GuildRoleDelete(in.GuildID, role.ID)
					if err != nil {
						gLogger.Println(err)
					}
					
					return
				}
				
				// time.Sleep(15 * time.Second)
				// err = s.ChannelMessageDelete(channel.ID, message.ID)
				// if err != nil {
				// 	gLogger.Println(err)
				// }
				
				
				
				err = s.MessageReactionAdd(channel.ID, message.ID, DEFAULT_REACTION_EMOJI)
				if err != nil {
					gLogger.Println(err)
				}
				
				s.InteractionRespond(in.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "```asciidoc\nSuccess :: The slash command was successful.\n```",
						Flags: 1 << 6,
					},
				})
				
				naq.aq.guildID = in.GuildID
				naq.aq.channelID = channel.ID
				naq.aq.messageID = message.ID
				naq.aq.roleID = role.ID
				
				gUTNewAutoQueries <- naq
				
				// var embed = utNewEmbed(naq.qe)
				
				// naq.aq.roleID = strconv.FormatBool(mentions)//CAUTION: USING VARIABLE AS UNION.
				// naq.aq.messageID = net.JoinHostPort(sHost, strconv.Itoa(iPort))//CAUTION: USING VARIABLE AS UNION.
				// naq.aq.channelID = channel.ID//channelName//CAUTION: USING VARIABLE AS UNION.
				
				// gUTAutoQuerySlashCommandUpdates <- aq
				// gLogger.Println(naq.aq)
				// go sendSelfDestructMessage(channel.ID, "Hello, world!", time.Duration(15 * time.Second))
				
				
				// gUTNewAutoQueries <- naq
			}
		}
	}
}

func messageReactionAddHandler(s *discordgo.Session, re *discordgo.MessageReactionAdd) {
	if utMessageReactionAddRelevant(re) {
		gMessageReactionAdds <- *re
	}
}

func messageReactionRemoveHandler(s *discordgo.Session, re *discordgo.MessageReactionRemove) {
	if utMessageReactionRemoveRelevant(re) {
		gMessageReactionRemoves <- *re
	}
}

func selfDestructMessageLoop(exit chan struct{}, newMessages chan SelfDestructMessage) {
	var timer = time.NewTimer(SELF_DESTRUCT_BACKUP_TIMER)
	var messages = make(map[string]SelfDestructMessage)
	loadSelfDestructMessages(messages)
	for {
		select {
		// case <- gExit:
		case <- exit:
			saveSelfDestructMessages(messages)
			return
			
		case <- timer.C:
			saveSelfDestructMessages(messages)
			
		case sdm := <- gSelfDestructMessages:
			if _, ok := messages[sdm.messageID]; ok {
				delete(messages, sdm.messageID)
			} else {
				messages[sdm.messageID] = sdm
			}
		}
	}
}

func loadSelfDestructMessages(messages map[string]SelfDestructMessage) {
	var file, err = os.Open("messages.csv")
	if err != nil {
		gLogger.Println(err)
		return
	}
	defer file.Close()
	
	var reader = csv.NewReader(file)
	reader.FieldsPerRecord = 3
	records, err := reader.ReadAll()
	if err != nil {
		gLogger.Println(err)
		return
	}
	
	var now = time.Now().UTC()
	
	for i, v := range records {
		var date, err = time.Parse(time.RFC3339, v[2])
		if err != nil {
			gLogger.Println(err)
			continue
		}
		messages[v[1]] = SelfDestructMessage{channelID: v[0], messageID: v[1], date: v[2]}
		
		var dur time.Duration
		if date.After(now) {
			dur = date.Sub(now)
		} else {
			dur = time.Duration((i * 10) % (60 * 60)) * time.Second//STAGGER DELETION TO AVOID RATE LIMITS.
		}
		
		go func(channelID string, messageID string, dur time.Duration) {
			time.Sleep(dur)
			var err = gBot.ChannelMessageDelete(channelID, messageID)
			if err != nil {
				gLogger.Println(err)
			}
			gSelfDestructMessages <- SelfDestructMessage{channelID: channelID, messageID: messageID, date: time.Now().UTC().Format(time.RFC3339)}
		}(v[0], v[1], dur)
	}
}

func saveSelfDestructMessages(messages map[string]SelfDestructMessage) {
	var file, err = os.Create("messages.csv")
	if err != nil {
		gLogger.Println(err)
		return
	}
	defer file.Close()
	
	var writer = csv.NewWriter(file)
	var records = make([][]string, 0, len(messages))
	for k, v := range messages {
		records = append(records, []string{v.channelID, k, v.date})
	}
	err = writer.WriteAll(records)
	if err != nil {
		gLogger.Println(err)
	}
}

func sendSelfDestructMessage(channelID string, message string, timer time.Duration) {
	var t = time.NewTimer(timer)
	m, err := gBot.ChannelMessageSend(channelID, message)
	if err != nil {
		gLogger.Println(err)
		return
	}
	var sdm = SelfDestructMessage{channelID: m.ChannelID, messageID: m.ID, date: time.Now().UTC().Add(timer).Format(time.RFC3339)}
	gSelfDestructMessages <- sdm
	
	<- t.C
	
	err = gBot.ChannelMessageDelete(m.ChannelID, m.ID)
	if err != nil {
		gLogger.Println(err)
	}
	
	gSelfDestructMessages <- sdm
}