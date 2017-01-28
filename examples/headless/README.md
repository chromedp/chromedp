# About headless

This is a version of the simple example but with the chromedp settings changed
to use the docker [yukinying/chrome-headless](yukinying/chrome-headless) image.

## Running

```sh
# retrieve docker image
docker pull yukinying/chrome-headless

# start docker headless
docker run -i -t --shm-size=256m --rm --name=chrome-headless -p=127.0.0.1:9222:9222 yukinying/chrome-headless about:blank

# run chromedp headless example
go build && ./headless
```
