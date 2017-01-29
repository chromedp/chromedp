# About headless

This is a version of the simple example but with the chromedp settings changed
to use the docker [knqz/chrome-headless](https://hub.docker.com/r/knqz/chrome-headless/) image.

## Running

```sh
# retrieve docker image
docker pull knqz/chrome-headless

# start chrome-headless
docker run -d -p 9222:9222 --rm --name chrome-headless knqz/chrome-headless

# run chromedp headless example
go build && ./headless
```
