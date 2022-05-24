// Copyright (C) 2017 Tulir Asokan
// Copyright (C) 2018-2020 Luca Weiss
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

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

var txtFile = flag.String("txt", "", "Path to welcome message file")
var htmlFile = flag.String("html", "", "Path to welcome message file")

func main() {
	flag.Parse()
	homeserver := os.Getenv(envHomeserver)
	username := os.Getenv(envUsername)
	password := os.Getenv(envPassword)
	roomID := os.Getenv(envRoomID)

	if homeserver == "" {
		errLog.Fatalf("%s is undefined", envHomeserver)
	}
	if username == "" {
		errLog.Fatalf("%s is undefined", envUsername)
	}
	if password == "" {
		errLog.Fatalf("%s is undefined", envPassword)
	}

	if roomID == "" || !(strings.HasPrefix(roomID, "!") || strings.HasPrefix(roomID, "#")) {
		log.Fatalf("%s is undefined, or wrong format", envRoomID)
	}
	if *htmlFile == "" {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *txtFile == "" {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	txtMsg, err := os.ReadFile(*txtFile)
	if err != nil {
		errLog.Fatalf("Could not read from %s", *txtFile)
	}
	htmlMsg, err := os.ReadFile(*htmlFile)
	if err != nil {
		errLog.Fatalf("Could not read from %s", *txtFile)
	}

	infoLog.Println("Logging into", homeserver, "as", username)
	client, err := mautrix.NewClient(homeserver, "", "")
	if err != nil {
		panic(err)
	}
	_, err = client.Login(&mautrix.ReqLogin{
		Type:             "m.login.password",
		Identifier:       mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: username},
		Password:         password,
		StoreCredentials: true,
	})
	if err != nil {
		panic(err)
	}
	infoLog.Println("Login successful")

	var room id.RoomID
	if strings.HasPrefix(roomID, "#") {
		resp, err := client.ResolveAlias(id.RoomAlias(roomID))
		if err != nil {
			errLog.Fatalf("Error: Could not find the room: %s\n", err)
		}
		room = resp.RoomID
	} else {
		room = id.RoomID(roomID)
	}

	syncer := client.Syncer.(*mautrix.DefaultSyncer)
	ignore := mautrix.OldEventIgnorer{
		UserID: client.UserID,
	}
	ignore.Register(syncer)
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
		if evt.Content.Raw["membership"] == "join" && evt.RoomID == room {
			rsp, err := client.CreateRoom(&mautrix.ReqCreateRoom{
				Preset:   "private_chat",
				Invite:   []id.UserID{evt.Sender},
				IsDirect: true,
			})
			if err != nil {
				errLog.Fatalf("Could not create instant messaging room: %s", err)
			}
			client.SendMessageEvent(rsp.RoomID, event.EventMessage, struct {
				Body          string `json:"body"`
				MsgType       string `json:"msgtype"`
				Format        string `json:"format"`
				FormattedBody string `json:"formatted_body"`
			}{
				Body:          string(txtMsg),
				MsgType:       "m.text",
				Format:        "org.matrix.custom.html",
				FormattedBody: string(htmlMsg),
			})
		}
	})

	err = client.Sync()
	if err != nil {
		panic(err)
	}
}
