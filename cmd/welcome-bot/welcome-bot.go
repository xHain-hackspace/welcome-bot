package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/xHain-hackspace/welcome-bot/internal/config"
	"github.com/xHain-hackspace/welcome-bot/internal/util"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const (
	envHomeserver = "WELCOME_BOT_HOMESERVER"
	envUsername   = "WELCOME_BOT_USERNAME"
	envPassword   = "WELCOME_BOT_PASSWORD"
	envRoomID     = "WELCOME_BOT_ROOM_ID"
)

const flags = log.Ldate | log.Ltime | log.Lmsgprefix

var errLog = log.New(os.Stderr, "[ERROR] ", flags)
var infoLog = log.New(os.Stdout, "[INFO] ", flags)

var configFile = flag.String("config", "", "Path to config file")

func main() {
	flag.Parse()

	// load config file
	if *configFile == "" {
		errLog.Printf("Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	conf, err := config.Parse(*configFile)

	// validate roomID format for rooms to watch
	validRomms := make([]string, 0)
	for _, id := range conf.Rooms {
		if !strings.HasPrefix(id, "!") && !strings.HasPrefix(id, "#") {
			errLog.Printf("RoomID '%s' is not valid, ignoring this room\n", id)
		} else {
			validRomms = append(validRomms, id)
		}
	}
	if len(validRomms) == 0 {
		errLog.Fatalf("No valid roomIDs have been provided, exiting...\n")
	}
	conf.Rooms = validRomms

	// ignore redirection if room is not given
	if conf.RedirectRoom == "" {
		conf.RedirectMessages = false
		errLog.Println("Redirection of Messages configured, but no room given!")
	}

	// validate roomID format for rooms to watch
	if !strings.HasPrefix(conf.RedirectRoom, "!") && !strings.HasPrefix(conf.RedirectRoom, "#") {
		errLog.Fatalf("RoomID '%s' is not valid!\n", conf.RedirectRoom)
	}

	// read messages
	txtMsg, err := os.ReadFile(conf.TxtMsgPath)
	if err != nil {
		errLog.Fatalf("Could not read from %s\n", conf.TxtMsgPath)
	}
	htmlMsg, err := os.ReadFile(conf.HtmlMsgPath)
	if err != nil {
		errLog.Fatalf("Could not read from %s\n", conf.HtmlMsgPath)
	}

	infoLog.Println("Logging into", conf.Homeserver, "as", conf.Username)
	client, err := mautrix.NewClient(conf.Homeserver, "", "")
	if err != nil {
		errLog.Fatal(err)
	}
	_, err = client.Login(&mautrix.ReqLogin{
		Type:             "m.login.password",
		Identifier:       mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: conf.Username},
		Password:         conf.Password,
		StoreCredentials: true,
	})
	defer func() {
		_, err := client.Logout()
		errLog.Println(err)
	}()
	if err != nil {
		errLog.Fatal(err)
	}
	infoLog.Println("Login successful")

	// validate roomIDs
	roomsToWatch := make([]id.RoomID, 0)
	for _, rid := range conf.Rooms {
		if strings.HasPrefix(rid, "#") {
			resp, err := client.ResolveAlias(id.RoomAlias(rid))
			if err != nil {
				errLog.Printf("Error: Could not find the room: %s\n", err)
			}
			roomsToWatch = append(roomsToWatch, resp.RoomID)
		} else {
			roomsToWatch = append(roomsToWatch, id.RoomID(rid))
		}
	}
	if len(roomsToWatch) == 0 {
		errLog.Fatalf("Could not resolve or find any of the provided rooms!\n")
	}

	var redirectRoom id.RoomID
	if conf.RedirectMessages {
		// validate redirectRoom
		if strings.HasPrefix(conf.RedirectRoom, "#") {
			resp, err := client.ResolveAlias(id.RoomAlias(conf.RedirectRoom))
			if err != nil {
				errLog.Fatalf("Error: Could not find the room: %s\n", err)
			}
			redirectRoom = resp.RoomID
		}
		// try to join the room
		if _, err := client.JoinRoomByID(redirectRoom); err != nil {
			log.Fatal(err)
		}
	}

	syncer := client.Syncer.(*mautrix.DefaultSyncer)
	ignore := mautrix.OldEventIgnorer{
		UserID: client.UserID,
	}
	ignore.Register(syncer)
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
		// auto join
		memberEvt := evt.Content.AsMember()
		if memberEvt.Membership == event.MembershipInvite && id.UserID(*evt.StateKey) == client.UserID {
			if _, err = client.JoinRoomByID(evt.RoomID); err != nil {
				errLog.Printf("Could not join room: %s\n", err)
				return
			} else {
				infoLog.Printf("Joined %s, invited by %s", string(evt.RoomID), evt.Sender.String())
			}
			return
		}
		// greet newly joined people
		if memberEvt.Membership == event.MembershipJoin && util.Contains(roomsToWatch, evt.RoomID) {
			rsp, err := client.CreateRoom(&mautrix.ReqCreateRoom{
				Preset:   "private_chat",
				Invite:   []id.UserID{evt.Sender},
				IsDirect: true,
			})
			if err != nil {
				errLog.Printf("Could not create instant messaging room: %s\n", err)
				return
			}
			err = client.SetAccountData("m.direct", &event.DirectChatsEventContent{evt.Sender: []id.RoomID{rsp.RoomID}})
			if err != nil {
				errLog.Println(err)
			}
			_, err = client.SendMessageEvent(rsp.RoomID, event.EventMessage, event.MessageEventContent{
				Body:          string(txtMsg),
				MsgType:       "m.text",
				Format:        "org.matrix.custom.html",
				FormattedBody: string(htmlMsg),
			})
			if err != nil {
				errLog.Printf("Could not send message: %s\n", err)
				infoLog.Printf("Leaving direct message with %s\n", evt.Sender)
				if _, err = client.LeaveRoom(rsp.RoomID); err != nil {
					errLog.Println(err)
					return
				}
				return
			}
			return
		}
	})

	if conf.RedirectMessages {
		infoLog.Println("Redirecting messages to: " + redirectRoom.String())
		syncer.OnEventType(event.EventMessage, func(source mautrix.EventSource, evt *event.Event) {
			// filter messages from redirect room
			if evt.RoomID == redirectRoom {
				return
			}
			// check message came from direct message
			direct := event.DirectChatsEventContent{}
			if err := client.GetAccountData("m.direct", &direct); err != nil {
				errLog.Println(err)
				return
			}
			if direct[evt.Sender] != nil && util.Contains(direct[evt.Sender], evt.RoomID) {
				msg := evt.Content.AsMessage()
				client.SendMessageEvent(redirectRoom, event.EventMessage, event.MessageEventContent{
					MsgType:       msg.MsgType,
					Body:          evt.Sender.String() + ": " + msg.Body,
					Format:        msg.Format,
					FormattedBody: msg.FormattedBody,
					URL:           msg.URL,
					Info:          msg.Info,
					File:          msg.File,
					NewContent:    msg.NewContent,
					RelatesTo:     msg.RelatesTo,
				})
			}
		})
	}
	err = client.Sync()
	if err != nil {
		errLog.Fatal(err)
	}
}
