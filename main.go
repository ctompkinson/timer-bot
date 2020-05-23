package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	Token  string
	Timers map[string]map[string]*Timer
)

type Timer struct {
	Time      time.Duration
	End       time.Time
	TimerName string
	Message   *discordgo.Message
	Stop      chan bool
}

func main() {
	Token = os.Getenv("DISCORD_TOKEN")
	if Token == "" {
		log.Println("Token not found please set env variables DISCORD_TOKEN")
	}

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Println("error creating Discord session, ", err)
	}

	if Timers == nil {
		Timers = make(map[string]map[string]*Timer)
	}

	dg.AddHandler(getMessageHandler)

	err = dg.Open()
	if err != nil {
		log.Println("error creating Discord session, ", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

func getMessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if !strings.Contains(m.Message.Content, "/timer") {
		return
	}

	parts := strings.Split(m.Message.Content, " ")
	if len(parts) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Must use verb set, list or cancel")
		return
	}

	switch parts[1] {
	case "set":
		t := Timer{Stop: make(chan bool, 2)}
		if Timers[m.GuildID] == nil {
			Timers[m.GuildID] = map[string]*Timer{}
		}

		name := strings.Join(parts[3:], " ")
		if Timers[m.GuildID][name] != nil {
			s.ChannelMessageSend(m.ChannelID, "A timer with that name already exists")
			return
		}

		Timers[m.GuildID][name] = &t
		go setCommand(s, m, &t)
	case "list":
		go listCommand(s, m)
	case "cancel":
		go cancelCommand(s, m)
	case "help":
		go helpCommand(s, m)
	default:
		s.ChannelMessageSend(m.ChannelID, "Must use verb set, list or cancel")
	}
	return
}

func setCommand(s *discordgo.Session, m *discordgo.MessageCreate, timer *Timer) {
	// !timer set 1m @everyone come on down
	// Created timer with id [ ] for 1m
	parts := strings.Split(m.Message.Content, " ")
	if !(len(parts) > 3) {
		s.ChannelMessageSend(m.ChannelID, "Please use format !timer set <time> <message>")
		return
	}

	rawTime := parts[2]
	d, err := time.ParseDuration(rawTime)
	if err != nil {
		fmt.Println(err)
		s.ChannelMessageSend(m.ChannelID, "Please use format !timer set <time> <message>")
		return
	}
	timer.Time = d
	timer.End = time.Now().Add(d)

	timer.TimerName = strings.Join(parts[3:], " ")

	r, err := s.ChannelMessageSend(
		m.ChannelID,
		fmt.Sprintf("%s has %s remaining", timer.TimerName, time.Now().Sub(timer.End).Round(time.Second).String()),
	)
	if err != nil {
		fmt.Println(err)
		s.ChannelMessageSend(m.ChannelID, "Please use format !timer set <time> <message>")
		return
	}
	timer.Message = r

	for {
		if time.Now().After(timer.End) {
			break
		}

		select {
		case <-timer.Stop:
			s.ChannelMessageEdit(m.ChannelID, timer.Message.ID,
				fmt.Sprintf("%s has been cancelled", timer.TimerName))
			delete(Timers[m.ChannelID], timer.TimerName)
			return
		default:
			s.ChannelMessageEdit(m.ChannelID, timer.Message.ID,
				fmt.Sprintf("%s has %s remaining", timer.TimerName, timer.End.Sub(time.Now()).Round(time.Second).String()))
			time.Sleep(time.Second)
		}
	}

	s.ChannelMessageEdit(m.ChannelID, timer.Message.ID, fmt.Sprintf("@everyone %s is over!", timer.TimerName))
	delete(Timers[m.ChannelID], timer.TimerName)
	return
}

func listCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	var keys []string
	for k, _ := range Timers[m.ChannelID] {
		keys = append(keys, k)
	}
	s.ChannelMessageSend(m.ChannelID, strings.Join(keys, "\n"))
	return
}

func cancelCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	parts := strings.Split(m.Message.Content, " ")

	id := strings.Join(parts[2:], " ")
	if Timers[m.GuildID] == nil {
		s.ChannelMessageSend(m.ChannelID, "Timer does not exist")
		return
	}

	if Timers[m.GuildID][id] == nil {
		s.ChannelMessageSend(m.ChannelID, "Timer does not exist")
		return
	}

	Timers[m.GuildID][id].Stop <- true
	return
}

func helpCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, "/timer set 1m name\n/timer cancel name\n/timer list")
	return
}
