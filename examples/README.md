This directory currently only contains an early example of using faceloader as a CLI tool

## Configuration

```shell
# set your page's url
echo 'FacebookPage: "https://www.facebook.com/myfacebookpage/events"' > .faceloader.yaml

# find out where chrome is on your computer
find / -type d -name "*Chrome.app"
# on linux this is probably:
echo 'ChromePath: "/usr/bin/chrome"' >> .faceloader.yaml

# on a mac this is probably:
echo 'ChromePath: "/System/Volumes/Data/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome"' >> .faceloader.yaml

# set your Facebook credentials in the config file
echo 'Username: "alice@example.org"' >> .faceloader.yaml
echo 'Password: "T0pS3cr3t"' >> .faceloader.yaml
```

Alternatively, you can store your `.faceloader.yaml` file in your home directory.  `.faceloader.json` or `.faceloader.toml` also work if you prefer

Or you can use environment variables:

```shell
export FACEBOOKPAGE="https://www.facebook.com/myfacebookpage/events"
export USERNAME="alice@example.com"
export PASSWORD="T0ps3cr3t"
```

You can set Debug=true using any of these methods to get a lot more information about what Chrome is doing.  But note that this is very verbose, and may include sensitive information.

## Running

To run the development version in this repo:

```shell
go run faceloader.go > calendar.ics
```

Run `go build` to create a binary from the source code