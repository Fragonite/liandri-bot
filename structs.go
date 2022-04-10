package main

import (
	"time"
)

type UTAutoQuery struct {
	guildID string
	channelID string
	messageID string
	roleID string
	// mentions bool
	joinMessages bool
	leaveMessages bool
	timer time.Duration
}

type UTAutoQueryLimitCheck struct {
	queryAddress string
	guildID string
	err chan error
}

type UTAutoQueryLoopUpdate struct {
	queryAddress string
	delete bool
}

type UTQueryEvent struct {
	queryAddress string
	online bool
	numTeams uint
	numPlayers uint
	ut UTQueryResult
}

type UTQueryResult struct {
	Hostname string
	Hostport string
	NumPlayers string
	MaxPlayers string
	GameType string
	MapName string
	// Spectators string
	GameStyle string
	TimeLimit string
	RemainingTime string
	GoalTeamScore string
	MapTitle string
	Teams [UT_MAX_TEAM_SIZE]UTQueryTeam
	Players [UT_MAX_PLAYER_SIZE]UTQueryPlayer
}

type UTQueryTeam struct {
	TeamName string
	TeamScore string
}

type UTQueryPlayer struct {
	Player string
	CountryC string
	Ping string
	Time string
	Frags string
	Deaths string
	Spree string
	Team string
	Mesh string
}

type UTNewAutoQuery struct {
	aq UTAutoQuery
	qe UTQueryEvent
}

type SelfDestructMessage struct {
	channelID string
	messageID string
	date string
}