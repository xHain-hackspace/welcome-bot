# Matrix Welcome Bot

This bot welcomes newly joined participants. It watches a specific room, which can be set via environment variable.
If a new user joins the bot will send a private (direct-)message to the newly joined user. The body of the message has to be provided in two seperate files: One html file and one plain text. Please note: The room the bot watches can not be encrypted.

## Usage

You have to set a couple of Environment Variables to make the bot work:

| Variable                 | Description                                                  |
| ------------------------ | ------------------------------------------------------------ |
| `WELCOME_BOT_HOMESERVER` | URL in the form of `matrix.example.org`                      |
| `WELCOME_BOT_USERNAME`   | Username of the bot user                                     |
| `WELCOME_BOT_PASSWORD`   | Password of the bot user                                     |
| `WELCOME_BOT_ROOM_ID`    | ID of the Room that will be used to send commands to the bot |

### Docker

To build the docker image run: `docker build -t welcome-bot .` from this directory.

An example docker run command can look like this:

```bash
docker run --rm -it \
  -v $LOCAL_PATH_TO_PLAIN_TEXT:/message.txt \
  -v $LOCAL_PATH_TO_HTML:/message.html \
  -e WELCOME_BOT_HOMESERVER=$SERVER_URL \
  -e WELCOME_BOT_USERNAME=$USERNAME \
  -e WELCOME_BOT_PASSWORD=$PASSWORD \
  -e WELCOME_BOT_ROOM_ID=$ROOMID \
  welcome-bot -txt /message.txt -html /message.html
```
