# About headless

This is a headless scraping example. It uses [knqz/chrome-headless](https://hub.docker.com/r/knqz/chrome-headless/)
docker image to operate. It provides basic way to get html and status code of a given page.

## Running

```sh
# retrieve docker image
docker pull knqz/chrome-headless

# start chrome-headless
docker run -d -p 9222:9222 --rm --name chrome-headless knqz/chrome-headless

# run chromedp headless example
go build && ./scrape
```
