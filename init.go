package main

import (
	"os"
	"log"
	"time"
	"strconv"
	"io"
	"strings"
	"github.com/bwmarrin/discordgo"
	"encoding/csv"
)

func init() {
	
	if gVersionLong == ASSERT_MISSING_VERSION {
		log.Fatal(gVersionLong)
	}
	if gVersionShort == ASSERT_MISSING_VERSION {
		log.Fatal(gVersionShort)
	}
	
	initPaths()
	initLogger()
	
	gLogger.Println("Liandri Bot " + gVersionLong)
	
	initAdminWhitelist()
	initDiscord()
	go selfDestructMessageLoop(gExit, gSelfDestructMessages)
	go utQueryLoop(gUTAutoQueryLoopUpdates, gUTQueryEvents)
	go utLoop(gUTQueryEvents, gMessageReactionAdds, gMessageReactionRemoves, gExit)
}

func initPaths() {
	err := os.MkdirAll("logs", 0777)
	if err != nil {
		log.Fatal(err)
	}
}

func initLogger() {
	var err error
	for i := 1; i <= 1000; i++ {
		gLogFile, err = os.OpenFile("logs/" + time.Now().Format("2006-01-02-") + strconv.Itoa(i) + ".log", os.O_RDWR | os.O_APPEND | os.O_CREATE | os.O_EXCL, 0666)
		if err == nil {
			gLogger = log.New(io.MultiWriter(os.Stdout, gLogFile), "liandri-bot-" + gVersionShort + ": ", log.Lshortfile | log.LstdFlags)
			return
		}
	}
	log.Fatal(err)
}

func initAdminWhitelist() {
	var file, err = os.OpenFile("admins.csv", os.O_RDONLY | os.O_CREATE, 0666)
	if err != nil {
		gLogger.Println(err)
		os.Exit(1)
		return
	}
	defer file.Close()

	var reader = csv.NewReader(file)
	reader.FieldsPerRecord = 1
	records, err := reader.ReadAll()
	if err != nil {
		gLogger.Println(err, "(admins.csv requires 1 ID per line)")
		os.Exit(1)
		return
	}

	for _, v := range records {
		gAdmins = append(gAdmins, v[0])
	}

	gLogger.Println("admins.csv:", gAdmins)
}

func initURLs() {
	var file, err = os.OpenFile("urls.csv", os.O_RDONLY | os.O_CREATE, 0666)
	if err != nil {
		gLogger.Println(err)
		os.Exit(1)
		return
	}
	defer file.Close()
	
	var reader = csv.NewReader(file)
	reader.FieldsPerRecord = 2
	records, err := reader.ReadAll()
	if err != nil {
		gLogger.Println(err, "(urls.csv requires 1 key-value pair per line)")
		os.Exit(1)
		return
	}
	
	for _, v := range records {
		gURLs.Store(v[0], v[1])
	}
}

func initDiscord() {
	var tokenFound = false
	var err error
	var slashCommandUpdates []string
	
	for i := 1; i < len(os.Args); i++ {
		switch {
		case strings.EqualFold(os.Args[i], "token"):
			if tokenFound {
				break
			}
			
			tokenFound = true
			
			i++
			if i < len(os.Args) {
				gBot, err = discordgo.New("Bot " + os.Args[i])
				if err != nil {
					gLogger.Println(err)
					os.Exit(1)
				}
				
				gBot.AddHandler(interactionCreateHandler)
				gBot.AddHandler(messageReactionAddHandler)
				gBot.AddHandler(messageReactionRemoveHandler)
				
				gBot.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessageReactions
				
				err = gBot.Open()
				if err != nil {
					gLogger.Println(err)
					os.Exit(1)
				}
			}
			
		case strings.EqualFold(os.Args[i], "slashcommand"):
			i++
			slashCommandUpdates = append(slashCommandUpdates, strings.Trim(os.Args[i], "\""))
		}
	}
	
	for _, id := range slashCommandUpdates {
		for _, command := range gSlashCommands {
			_, err = gBot.ApplicationCommandCreate(gBot.State.User.ID, id, command)
			if err != nil {
				gLogger.Println(err)
			}
		}
	}
	
	if !(tokenFound) {
		gLogger.Println("Token not found. Usage: token <token>")
		os.Exit(1)
	}
}