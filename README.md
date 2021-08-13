# Faceloader

A single binary to generate an ical file from a Facebook page

## Configuration

```shell
# set your page's url
echo 'FacebookPage: "https://www.facebook.com/myfacebookpage/events"' > config.yaml
# set the path to the chrome binary on your computer
echo 'ChromePath: "/usr/bin/chrome"' >> config.yaml
# configure the file to write events to once an hour
echo 'CalendarPath: "/tmp/events.ics"' >> config.yaml
```

## Running

Assuming you've downloaded a release or have built a binary you can simply run the executable:

```shell
./faceloader
```

If you want to build from source you can do so with:

```shell
go build
```

## Development

You may need to install some extra packages to build this:

```shell
sudo apt-get install gcc libgtk-3-dev libappindicator3-dev
```

Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) when writing your commit messages

To release, create and push a new tag and then run `goreleaser`