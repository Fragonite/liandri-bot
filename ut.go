package main

import (
	"net"
	"time"
	"strings"
	"strconv"
	"sort"
	"fmt"
	"errors"
	"github.com/bwmarrin/discordgo"
	"os"
	"encoding/csv"
	"unicode/utf8"
)

func utLoop(queryEvents chan UTQueryEvent, reactionAddEvents chan discordgo.MessageReactionAdd, reactionRemoveEvents chan discordgo.MessageReactionRemove, exit chan struct{}) {
	var utaq = make(map[string][]UTAutoQuery)
	var utqe = make(map[string]UTQueryEvent)
	var numPlayersPrevPrev = make(map[string]uint)
	
	utLoadAutoQueries(utaq)
	
	for key, _ := range utaq {
		utqe[key] = UTQueryEvent{}
		numPlayersPrevPrev[key] = 999//SUPPRESS MENTIONS ON CRASH OR RESTART.
		gUTAutoQueryLoopUpdates <- UTAutoQueryLoopUpdate{queryAddress: key, delete: false}//DEADLOCK POTENTIAL?
	}
	
	for {
		select {
		case qe := <- queryEvents:
			
			if _, ok := utaq[qe.queryAddress]; ok {
				if prev, ok := utqe[qe.queryAddress]; ok {
					
					// Reduce updates when the only players are spectators.
					for i, player := range qe.ut.Players {
						if player.Mesh == "Spectator" {
							qe.ut.Players[i].Ping = "0"
							qe.ut.Players[i].Time = "0"
						}
					}
					
					sort.Slice(qe.ut.Players[:qe.numPlayers], func (i, j int) bool {
						if qe.ut.Players[i].Mesh == "Spectator" || qe.ut.Players[j].Mesh == "Spectator" {
							return false
						}
						score0, err := strconv.Atoi(qe.ut.Players[i].Frags)
						if err != nil {
							gLogger.Println(qe.queryAddress, qe.ut.Players[i])
							return false
						}
						score1, err := strconv.Atoi(qe.ut.Players[j].Frags)
						if err != nil {
							gLogger.Println(qe.queryAddress, qe.ut.Players[j])
							return false
						}
						if score0 != score1 {
							return score0 > score1
						}
						time0, err := strconv.Atoi(qe.ut.Players[i].Time)
						if err != nil {
							gLogger.Println(err)
							return false
						}
						time1, err := strconv.Atoi(qe.ut.Players[j].Time)
						if err != nil {
							gLogger.Println(err)
							return false
						}
						return time0 < time1
					})
					
					if qe == prev {
						break
					}
					
					embed, err := utNewEmbed(qe)
					if err != nil {
						gLogger.Println(err)
						break
					}
					
					var status = utGetServerStatus(&qe)
					
					// Reduce chance of rate limits.
					if status == SERVER_STATUS_FULL {
						status = SERVER_STATUS_ACTIVE
					}
					if status == SERVER_STATUS_OFFLINE || (prev.numPlayers > 0 && qe.numPlayers == 0) {
						status = ""
					}
					
					var mention = numPlayersPrevPrev[qe.queryAddress] == 0 && prev.numPlayers == 0 && qe.numPlayers > 0
					
					for _, v := range utaq[qe.queryAddress] {
						go func (messageEmbed discordgo.MessageEmbed, queryAddress string, autoQuery UTAutoQuery, serverStatus string, mentionRole bool) {
							_, err := gBot.ChannelMessageEditEmbed(autoQuery.channelID, autoQuery.messageID, &messageEmbed)
							if err != nil {
								gLogger.Println(err)
								gLogger.Println("Deleting auto query channel and role.")
								gUTDeleteAutoQueries <- UTNewAutoQuery{aq: autoQuery, qe: UTQueryEvent{queryAddress: queryAddress}}
								return
							}
							
							if autoQuery.mentions > 0 && mentionRole {
								go sendSelfDestructMessage(autoQuery.channelID, "<@" + autoQuery.roleID + "> A new foe has appeared!", time.Duration(time.Minute * 10))
							}
							
							st, err := gBot.Channel(autoQuery.channelID)
							rune, _ := utf8.DecodeRuneInString(st.Name)
							switch(string(rune)) {
							case SERVER_STATUS_FULL:
								fallthrough
							case SERVER_STATUS_ACTIVE:
								fallthrough
							case SERVER_STATUS_ONLINE:
								fallthrough
							case SERVER_STATUS_OFFLINE:
								if status != "" && status != string(rune) {
									var edit = discordgo.ChannelEdit{
										Name: status + st.Name[1:],
										Position: st.Position,
									}
									_, err = gBot.ChannelEditComplex(autoQuery.channelID, &edit)
									if err != nil {
										gLogger.Println(err)
									}
								}
							}
						}(embed, qe.queryAddress, v, status, mention)
					}
					
					if qe.online {
						numPlayersPrevPrev[qe.queryAddress] = prev.numPlayers
						utqe[qe.queryAddress] = qe
					} else {
						prev.online = false
						utqe[qe.queryAddress] = prev
					}
				} else {
					gLogger.Println(ok)
				}
			}
			
		case naq := <- gUTNewAutoQueries:
			var queryAddress = naq.qe.queryAddress
			utaq[queryAddress] = append(utaq[queryAddress], naq.aq)
			utqe[queryAddress] = naq.qe
			gUTAutoQueryLoopUpdates <- UTAutoQueryLoopUpdate{queryAddress: queryAddress, delete: false}
			utSaveAutoQueries(utaq)
			
		case naq := <- gUTDeleteAutoQueries:
			var err error
			var queryAddress = naq.qe.queryAddress
			if aqs, ok := utaq[queryAddress]; ok {
				for i, v := range aqs {
					if v.channelID == naq.aq.channelID {
						aqs[i] = aqs[len(aqs) - 1]
						aqs = aqs[:len(aqs) - 1]
						if len(aqs) == 0 {
							delete(utaq, queryAddress)
							delete(utqe, queryAddress)
							gUTAutoQueryLoopUpdates <- UTAutoQueryLoopUpdate{queryAddress: queryAddress, delete: true}
						} else {
							utaq[queryAddress] = aqs
						}
						
						_, err = gBot.ChannelDelete(v.channelID)
						if err != nil {
							gLogger.Println(err)
						}
						
						err = gBot.GuildRoleDelete(v.guildID, v.roleID)
						if err != nil {
							gLogger.Println(err)
						}
						
						break
					}
				}
			}
			
		case lc := <- gUTAutoQueryLimitChecks:
			
			// gLogger.Println("Limit checking")

			var queryAddressDuplicate = false
			if _, ok := utaq[lc.queryAddress]; ok {
				for _, v := range utaq[lc.queryAddress] {
					if lc.guildID == v.guildID {
						queryAddressDuplicate = true
						break
					}
				}
			}
			
			if queryAddressDuplicate {
				lc.err <- errors.New("An entry for this server already exists.")
				break
			}
			
			// This could get expensive if spammed.
			var noError = true
			var numServers = 0
			multiBreakLimitCheck:
			for _, v := range utaq {
				for _, aq := range v {
					if lc.guildID == aq.guildID {
						numServers++
						if numServers >= MAX_AUTO_QUERY_NUMBER_PER_GUILD {
							lc.err <- errors.New(fmt.Sprintf("Server limit reached (%d).", MAX_AUTO_QUERY_NUMBER_PER_GUILD))
							noError = false
							break multiBreakLimitCheck
						}
					}
				}
			}
			
			if noError {
				lc.err <- nil
			}
			
			
		case re := <- reactionAddEvents:
			
			
			if !(utMessageReactionAddRelevant(&re)) {
				break
			}
			multiBreakReactionAdd:
			for key, aqs := range utaq {
				for i, v := range aqs {
					if v.messageID == re.MessageID {
						err := gBot.GuildMemberRoleAdd(v.guildID, re.UserID, v.roleID)
						if err != nil {
							gLogger.Println(err, re)
						}
						v.mentions += 1
						utaq[key][i] = v
						break multiBreakReactionAdd
					}
				}
			}
			
			
		case re := <- reactionRemoveEvents:
			
			
			if !(utMessageReactionRemoveRelevant(&re)) {
				break
			}
			multiBreakReactionRemove:
			for key, aqs := range utaq {
				for i, v := range aqs {
					if v.messageID == re.MessageID {
						err := gBot.GuildMemberRoleRemove(v.guildID, re.UserID, v.roleID)
						if err != nil {
							gLogger.Println(err, re)
						}
						v.mentions = MaxInt(v.mentions - 1, 0)
						utaq[key][i] = v
						break multiBreakReactionRemove
					}
				}
			}
			
		case <- exit:
			utSaveAutoQueries(utaq)
			return
		}
	}
}

func utQueryLoop(chIn <- chan UTAutoQueryLoopUpdate, chOut chan <- UTQueryEvent) {
	type ServerList struct {
		queryAddress string
		waitPeriod uint
		delayPeriod uint
		instanceCount uint
	}
	var serverList = make([]ServerList, 0, 100)
	var ticker = time.NewTicker(10000 * time.Millisecond)
	var queryEvents = make(chan UTQueryEvent, 100)
	
	for {
		select {
		//QUERY ALL SERVERS STAGGERED EVENLY OVER 10 SECONDS, SKIPPING SERVERS WITH ACTIVE WAIT PERIODS.
		case <- ticker.C:
			for i, v := range serverList {
				if v.waitPeriod == 0 {
					serverList[i].waitPeriod = v.delayPeriod
					go func(sleep time.Duration, queryAddress string) {
						time.Sleep(sleep)
						queryEvents <- utNewQueryEvent(queryAddress, utQueryServer(queryAddress))
					}(time.Duration(10000 / len(serverList) * i) * time.Millisecond, v.queryAddress)
				} else {
					serverList[i].waitPeriod--
				}
			}
			
		//SEND BACK RESULT OF QUERY, UPDATE DELAY PERIOD, CAP WAIT PERIOD TO DELAY PERIOD.
		case event := <- queryEvents:
			chOut <- event
			
			var i = sort.Search(len(serverList), func(i int) bool {
				return serverList[i].queryAddress >= event.queryAddress
			})
			if i < len(serverList) && serverList[i].queryAddress == event.queryAddress {
				
				if event.online {
					if event.ut.NumPlayers == "0" {
						serverList[i].delayPeriod = 1
						serverList[i].waitPeriod = MinUint(serverList[i].waitPeriod, serverList[i].delayPeriod)
					} else {
						serverList[i].delayPeriod = 0
						serverList[i].waitPeriod = 0
					}
				} else {
					serverList[i].delayPeriod = MinUint(serverList[i].delayPeriod + 1, 5)
				}
			}
			
		//ADD OR REMOVE SERVER LIST ENTRIES.
		case update := <- chIn:
			if update.delete == false {
				
				var i = sort.Search(len(serverList), func(i int) bool {
					return serverList[i].queryAddress >= update.queryAddress
				})
				if i < len(serverList) && serverList[i].queryAddress == update.queryAddress {
					serverList[i].instanceCount++
				} else {
					serverList = append(serverList, ServerList{
						queryAddress: update.queryAddress, waitPeriod: 0, delayPeriod: 0, instanceCount: 1,
					})
				}
				
			} else {
				
				var i = sort.Search(len(serverList), func(i int) bool {
					return serverList[i].queryAddress >= update.queryAddress
				})
				if i < len(serverList) && serverList[i].queryAddress == update.queryAddress {
					serverList[i].instanceCount--
					if serverList[i].instanceCount == 0 {
						serverList[i] = serverList[len(serverList) - 1]
						serverList = serverList[:len(serverList) - 1]
					}
				}
				
			}
			
			sort.Slice(serverList, func(i, j int) bool {
				return serverList[i].queryAddress < serverList[j].queryAddress
			})
		}
	}
}

func utQueryServer(queryAddress string) string {
	conn, err := net.Dial("udp", queryAddress)
	if err != nil {
		gLogger.Println(err)
		return ""
	}
	defer conn.Close()
	
	err = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		gLogger.Println(err)
		return ""
	}
	
	_, err = conn.Write([]byte("\\status\\XServerQuery"))
	if err != nil {
		gLogger.Println(err)
		return ""
	}
	
	var result = make([]byte, 1024 * 4)
	var finalResult = make([]byte, 0, 1024 * 8)
	
	for {
		// err = conn.SetDeadline(time.Now().Add(5 * time.Second))
		// if err != nil {
		// 	gLogger.Println(err)
		// 	return ""
		// }
		
		n, err := conn.Read(result)
		if err != nil {
			// gLogger.Println(err)
			return ""
		}
		
		finalResult = append(finalResult, result[:n]...)
		
		if strings.EqualFold(LastN(string(finalResult), 7), "\\final\\") {
			return string(finalResult)
		}
	}
}

func utNewQueryEvent(queryAddress, queryResult string) UTQueryEvent {
	var qe = UTQueryEvent{queryAddress: queryAddress}
	
	if queryResult == "" {
		return qe
	}
	
	var s = strings.Split(queryResult, "\\")
	for i := 0; i < len(s); i++ {
		switch {
		case strings.EqualFold(s[i], "Hostname"):
			i++
			qe.ut.Hostname = s[i]
		case strings.EqualFold(s[i], "Hostport"):
			i++
			qe.ut.Hostport = s[i]
		case strings.EqualFold(s[i], "NumPlayers"):
			i++
			qe.ut.NumPlayers = s[i]
		case strings.EqualFold(s[i], "MaxPlayers"):
			i++
			qe.ut.MaxPlayers = s[i]
		case strings.EqualFold(s[i], "GameType"):
			i++
			qe.ut.GameType = s[i]
		case strings.EqualFold(s[i], "MapName"):
			i++
			qe.ut.MapName = s[i]
		// case strings.EqualFold(s[i], "Spectators"):
		// 	i++
		// 	qe.ut.Spectators = s[i]
		case strings.EqualFold(s[i], "GameStyle"):
			i++
			qe.ut.GameStyle = s[i]
		case strings.EqualFold(s[i], "TimeLimit"):
			i++
			qe.ut.TimeLimit = s[i]
		case strings.EqualFold(s[i], "RemainingTime"):
			i++
			qe.ut.RemainingTime = s[i]
		case strings.EqualFold(s[i], "GoalTeamScore"):
			i++
			qe.ut.GoalTeamScore = s[i]
		case strings.EqualFold(s[i], "MapTitle"):
			i++
			qe.ut.MapTitle = s[i]
			
		case strings.EqualFold(FirstN(s[i], 9), "TeamName_"):
			team, err := strconv.ParseUint(s[i][9:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if team > UT_MAX_TEAM_SIZE {
				gLogger.Println(team)
				break
			}
			qe.ut.Teams[team].TeamName = s[i]
			if uint(team) + 1 > qe.numTeams {
				qe.numTeams = uint(team) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 10), "TeamScore_"):
			team, err := strconv.ParseUint(s[i][10:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if team > UT_MAX_TEAM_SIZE {
				gLogger.Println(team)
				break
			}
			qe.ut.Teams[team].TeamScore = s[i]
			if uint(team) + 1 > qe.numTeams {
				qe.numTeams = uint(team) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 7), "Player_"):
			player, err := strconv.ParseUint(s[i][7:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Player = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 9), "CountryC_"):
			player, err := strconv.ParseUint(s[i][9:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].CountryC = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 5), "Ping_"):
			player, err := strconv.ParseUint(s[i][5:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Ping = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 5), "Time_"):
			player, err := strconv.ParseUint(s[i][5:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Time = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 6), "Frags_"):
			player, err := strconv.ParseUint(s[i][6:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Frags = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 7), "Deaths_"):
			player, err := strconv.ParseUint(s[i][7:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Deaths = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 6), "Spree_"):
			player, err := strconv.ParseUint(s[i][6:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Spree = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 5), "Team_"):
			player, err := strconv.ParseUint(s[i][5:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Team = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
			
		case strings.EqualFold(FirstN(s[i], 5), "Mesh_"):
			player, err := strconv.ParseUint(s[i][5:], 10, 32)
			i++
			if err != nil {
				gLogger.Println(err)
				break
			}
			if player > UT_MAX_PLAYER_SIZE {
				gLogger.Println(player)
				break
			}
			qe.ut.Players[player].Mesh = s[i]
			if uint(player) + 1 > qe.numPlayers {
				qe.numPlayers = uint(player) + 1
			}
		}
	}
	
	if qe.ut.Hostport != "" {
		qe.online = true
	}
	
	return qe
}

func utNewEmbed(qe UTQueryEvent) (discordgo.MessageEmbed, error) {
	var embed discordgo.MessageEmbed
	var author discordgo.MessageEmbedAuthor
	var footer discordgo.MessageEmbedFooter
	
	sHost, sPort, err := net.SplitHostPort(qe.queryAddress)
	if err != nil {
		return embed, err
	}
	
	iPort, err := strconv.Atoi(sPort)
	if err != nil {
		return embed, err
	}
	
	footer.Text = "unreal://" + net.JoinHostPort(sHost, strconv.Itoa(iPort - 1)) + "⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀"
	embed.Author = &author
	embed.Footer = &footer
	
	if !(qe.online) {
		author.Name = "Query Server Timeout"
		if qe.ut.Hostname != "" {
			embed.Title = qe.ut.Hostname
		}
		embed.Color = EMBED_COLOUR_OFFLINE
		embed.Author = &author
		embed.Footer = &footer
		return embed, nil
	}
	
	var fields = make([]discordgo.MessageEmbedField, 0, 10)
	
	author.Name = qe.ut.NumPlayers + "/" + qe.ut.MaxPlayers + " Online"
	embed.Title = qe.ut.Hostname
	embed.Description = qe.ut.MapTitle + " • " + qe.ut.GameType + " • " + qe.ut.GameStyle
	
	if strings.EqualFold(qe.ut.NumPlayers, qe.ut.MaxPlayers) {
		embed.Color = EMBED_COLOUR_FULL
	} else if qe.numPlayers > 0 {
		embed.Color = EMBED_COLOUR_ACTIVE
	} else {
		embed.Color = EMBED_COLOUR_ONLINE
	}
	
	var scoreLimit = qe.ut.GoalTeamScore
	if scoreLimit == "" {
		scoreLimit = "⠀"
	}
	
	var remainingTime = FormatSecondsToMinutes(qe.ut.RemainingTime)
	if remainingTime == "" {
		remainingTime = "⠀"
	}
	
	fields = append(fields, discordgo.MessageEmbedField{Name: "Score Limit", Value: scoreLimit, Inline: true})
	fields = append(fields, discordgo.MessageEmbedField{Name: "Time Limit", Value: qe.ut.TimeLimit + ":00", Inline: true})
	fields = append(fields, discordgo.MessageEmbedField{Name: "Remaining Time", Value: remainingTime, Inline: true})
	
	switch qe.ut.GameType {
	case "DeathMatchPlus":
		var spectators, players, scores, pings strings.Builder
		spectators.Grow(16 * int(qe.numPlayers))
		players.Grow(16 * int(qe.numPlayers))
		scores.Grow(8 * int(qe.numPlayers))
		pings.Grow(8 * int(qe.numPlayers))
		
		for i := 0; i < int(qe.numPlayers); i++ {
			var player string
			
			if qe.ut.Players[i].CountryC == "" || qe.ut.Players[i].CountryC == "none" {
				player = ":flag_aq: " + qe.ut.Players[i].Player
			} else {
				player = ":flag_" + qe.ut.Players[i].CountryC + ": " + qe.ut.Players[i].Player
			}
			
			if qe.ut.Players[i].Mesh == "Spectator" {
				spectators.WriteString(player)
				spectators.WriteRune('\n')
			} else {
				players.WriteString(player)
				players.WriteRune('\n')
				
				scores.WriteString(qe.ut.Players[i].Frags)
				scores.WriteRune('\n')
				
				pings.WriteString(qe.ut.Players[i].Ping)
				pings.WriteRune('\n')
			}
			
		}
		
		if players.String() != "" {
			fields = append(fields, discordgo.MessageEmbedField{Name: "Players", Value: players.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Score", Value: scores.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Ping", Value: pings.String(), Inline: true})
		}
		
		if spectators.String() != "" {
			fields = append(fields, discordgo.MessageEmbedField{Name: "Spectators", Value: spectators.String(), Inline: true})
		}
		
	default:
		var spectators, team0, team1, team2, team3, scores0, scores1, scores2, scores3, pings0, pings1, pings2, pings3 strings.Builder
		spectators.Grow(16 * int(qe.numPlayers))
		team0.Grow(16 * int(qe.numPlayers))
		scores0.Grow(8 * int(qe.numPlayers))
		pings0.Grow(8 * int(qe.numPlayers))
		team1.Grow(16 * int(qe.numPlayers))
		scores1.Grow(8 * int(qe.numPlayers))
		pings1.Grow(8 * int(qe.numPlayers))
		
		for i := 0; i < int(qe.numPlayers); i++ {
			var player string
			
			if qe.ut.Players[i].CountryC == "" || qe.ut.Players[i].CountryC == "none" {
				player = ":flag_aq: " + qe.ut.Players[i].Player
			} else {
				player = ":flag_" + qe.ut.Players[i].CountryC + ": " + qe.ut.Players[i].Player
			}
			
			if qe.ut.Players[i].Mesh == "Spectator" {
				spectators.WriteString(player)
				spectators.WriteRune('\n')
			} else {
				switch qe.ut.Players[i].Team {
				case "0":
					team0.WriteString(player)
					team0.WriteRune('\n')
					scores0.WriteString(qe.ut.Players[i].Frags)
					scores0.WriteRune('\n')
					pings0.WriteString(qe.ut.Players[i].Ping)
					pings0.WriteRune('\n')
				case "1":
					team1.WriteString(player)
					team1.WriteRune('\n')
					scores1.WriteString(qe.ut.Players[i].Frags)
					scores1.WriteRune('\n')
					pings1.WriteString(qe.ut.Players[i].Ping)
					pings1.WriteRune('\n')
				case "2":
					team2.WriteString(player)
					team2.WriteRune('\n')
					scores2.WriteString(qe.ut.Players[i].Frags)
					scores2.WriteRune('\n')
					pings2.WriteString(qe.ut.Players[i].Ping)
					pings2.WriteRune('\n')
				case "3":
					team3.WriteString(player)
					team3.WriteRune('\n')
					scores3.WriteString(qe.ut.Players[i].Frags)
					scores3.WriteRune('\n')
					pings3.WriteString(qe.ut.Players[i].Ping)
					pings3.WriteRune('\n')
				}
			}
		}
		
		var team0Title string
		if score := qe.ut.Teams[0].TeamScore; score != "" {
			team0Title = "Red Team ` " + score + " `"
		} else {
			team0Title = "Red Team"
		}
		
		var team1Title string
		if score := qe.ut.Teams[1].TeamScore; score != "" {
			team1Title = "Blue Team ` " + score + " `"
		} else {
			team1Title = "Blue Team"
		}
		
		var team2Title string
		if score := qe.ut.Teams[2].TeamScore; score != "" {
			team2Title = "Green Team ` " + score + " `"
		} else {
			team2Title = "Green Team"
		}
		
		var team3Title string
		if score := qe.ut.Teams[3].TeamScore; score != "" {
			team3Title = "Gold Team ` " + score + " `"
		} else {
			team3Title = "Gold Team"
		}
		
		if team0.String() != "" {
			fields = append(fields, discordgo.MessageEmbedField{Name: team0Title, Value: team0.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Score", Value: scores0.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Ping", Value: pings0.String(), Inline: true})
		}
		
		if team1.String() != "" {
			fields = append(fields, discordgo.MessageEmbedField{Name: team1Title, Value: team1.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Score", Value: scores1.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Ping", Value: pings1.String(), Inline: true})
		}
		
		if team2.String() != "" {
			fields = append(fields, discordgo.MessageEmbedField{Name: team2Title, Value: team2.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Score", Value: scores2.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Ping", Value: pings2.String(), Inline: true})
		}
		
		if team3.String() != "" {
			fields = append(fields, discordgo.MessageEmbedField{Name: team3Title, Value: team3.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Score", Value: scores3.String(), Inline: true})
			fields = append(fields, discordgo.MessageEmbedField{Name: "Ping", Value: pings3.String(), Inline: true})
		}
		
		if spectators.String() != "" {
			fields = append(fields, discordgo.MessageEmbedField{Name: "Spectators", Value: spectators.String(), Inline: true})
		}
	}
	
	var pFields = make([]*discordgo.MessageEmbedField, len(fields))
	for i := range pFields {
		pFields[i] = &fields[i]
	}
	embed.Fields = pFields
	return embed, nil
}

func utGetServerStatus(qe *UTQueryEvent) (string) {
	switch {
	case !(qe.online):
		return SERVER_STATUS_OFFLINE
		
	case qe.numPlayers > 0:
		if qe.ut.NumPlayers == qe.ut.MaxPlayers {
			return SERVER_STATUS_FULL
		} else {
			return SERVER_STATUS_ACTIVE
		}
		
	default:
		return SERVER_STATUS_ONLINE
	}
}

func utMessageReactionAddRelevant(re *discordgo.MessageReactionAdd) (bool) {
	if re.UserID == gBot.State.User.ID {
		return false
	}
	
	if re.Member == nil {
		return false
	}
	
	if re.Emoji.Name != DEFAULT_REACTION_EMOJI {
		return false
	}
	
	return true
}

func utMessageReactionRemoveRelevant(re *discordgo.MessageReactionRemove) (bool) {
	if re.UserID == gBot.State.User.ID {
		return false
	}
	
	if re.Emoji.Name != DEFAULT_REACTION_EMOJI {
		return false
	}
	
	return true
}

func utSaveAutoQueries(utaq map[string][]UTAutoQuery) {
	var file, err = os.Create("ut.csv")
	if err != nil {
		gLogger.Println(err)
		return
	}
	defer file.Close()
	
	var writer = csv.NewWriter(file)
	var records = make([][]string, 0, len(utaq))
	
	for k := range utaq {
		var fields = make([]string, 0, len(utaq[k]) * 8 + 1)
		fields = append(fields, k)
		for _, v := range utaq[k] {
			fields = append(fields, []string{
				v.guildID, 
				v.channelID, 
				v.messageID, 
				v.roleID, 
				strconv.Itoa(v.mentions), 
				strconv.FormatBool(v.joinMessages), 
				strconv.FormatBool(v.leaveMessages), 
				v.timer.String(),
			}...)
		}
		records = append(records, fields)
	}
	err = writer.WriteAll(records)
	if err != nil {
		gLogger.Println(err)
	}
}

func utLoadAutoQueries(utaq map[string][]UTAutoQuery) {
	var file, err = os.Open("ut.csv")
	if err != nil {
		gLogger.Println(err)
		return
	}
	defer file.Close()
	
	var reader = csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		gLogger.Println(err)
		return
	}
	
	for _, r := range records {
		var key = r[0]
		for j := 1; j <= len(r[1:]); j += 8 {
			mentions, err := strconv.Atoi(r[j+4])
			if err != nil {
				gLogger.Println(err)
				mentions = 0
			}
			joinMessages, err := strconv.ParseBool(r[j+5])
			if err != nil {
				gLogger.Println(err)
				joinMessages = false
			}
			leaveMessages, err := strconv.ParseBool(r[j+6])
			if err != nil {
				gLogger.Println(err)
				leaveMessages = false
			}
			timer, err := time.ParseDuration(r[j+7])
			if err != nil {
				gLogger.Println(err)
				timer = SELF_DESTRUCT_TIMER_DEFAULT
			}
			utaq[key] = append(utaq[key], UTAutoQuery{
				guildID: r[j],
				channelID: r[j+1],
				messageID: r[j+2],
				roleID: r[j+3],
				mentions: mentions,
				joinMessages: joinMessages,
				leaveMessages: leaveMessages,
				timer: timer,
			})
		}
	}
}