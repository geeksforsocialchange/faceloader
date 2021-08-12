# Faceloader

A single binary to generate an ical file from a Facebook page

## Configuration

```shell
# set your page's url
echo 'FacebookPage: "https://www.facebook.com/myfacebookpage/events"' > config.yaml
# set the path to the chrome binary on your computer
echo 'ChromePath: "/usr/bin/chrome"' >> config.yaml
```

## Running

```shell
./faceloader > calendar.ics
```

## Development

Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) when writing your commit messages