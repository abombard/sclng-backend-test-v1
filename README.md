# Canvas for Backend Technical Test at Scalingo

## Instructions

* From this canvas, respond to the project which has been communicated to you by our team
* Feel free to change everything

## Execution

```
docker compose up
```

Application will be then running on port `5000`

## Test

```
$ curl localhost:5000/ping
{ "status": "pong" }
```

## Endpoints

A few parameters can be passed to both endpoints:

* Authenticate with your github token
```
$ curl -H "Authorization: Bearer <GITHUB_TOKEN>" localhost:5000/repos
```

* Instead of fetching the last 100 repositories, fetch 100 repositories from <id>
```
$ curl localhost:5000/repos?since=1
```

### Repository

Lists the last 100 repositories created.
```
$ curl localhost:5000/repos
  {
    "name": "hotwire",
    "url": "https://api.github.com/repos/zsx/hotwire",
    "owner": "zsx",
    "description": "The git repository of hotwire-shell",
  },
  ...
```

### Repository stats

Returns some stats on the last 100 repositories created

```
$ curl localhost:5000/stats
  {
    "name": "joy2chord",
    "url": "https://api.github.com/repos/holizz/joy2chord",
    "owner": "holizz",
    "description": "userspace input driver for chorded /dev/input/js* based keyers",
    "stars_count": 0,
    "languages": {
      "C++": 41809,
      "Shell": 96,
      "Vim Script": 1562
    }
  },
  ...
```

A few more parameters can be passed to this endpoint:

* Filters repositories based on the languages. Example: `Go`
Uses GET languages
```
$ curl localhost:5000/stats?language=Go

```

* Filters the repositories based on the license. Example: `mit`
Uses GET repository: `lincense.key`
```
$ curl localhost:5000/stats?license=mit

```

## Architecture

* /repos
Most of the code for this endpoint can be found in ./repository.go

This endpoint will fetch the last 100 repositories created using the list repository API.
Since the API pagination is rather limited, i did my best to find the last 100 with the least calls possible (~30 calls at the moment).
It still takes quite some time but well.
The algorithm uses a high / low bound where high = no result found, low = 100 repositories found and try to get closer to the correct id
by progressively raising the low bound and lowering the high bound.

* /stats
Most of the code for this endpoint can be found in ./repository_stats.go

Most of the processing of this endpoint is done in some goroutines spawned when the server starts.
Thoses goroutines wait for tasks to be queued in a channel.
The reason they are spawned when the server start is to avoid the extra cost of spawning them each time
the endpoint is called and have goroutines spawning with no limit based on how many call the endpoint
are made at the same time. This allows some more control over the resources of the server.

So once we fetched the 100 last repositories created, they are passed to the task queue channel.
The goroutines pick up the tasks, fetch some more information using the github API
- GET repository -> get the license
- GET languages -> get the language
Then filters out the repositories based on the query parameters if provided.
And complete the repository information before passing the results back through a channel.


## Notes

I've spent quite some time on this already so i'll just add a little list of things i would have done if i was fast or this was a repository i'd have to actually manage.
* clean up the query parameters and filtering. passing url.Values everywhere, not so clean
* add the number of workers and size of the task queue in conf
* make some performance tests to find out what's the best numbers for thoses (never done that before though)
* add some tests on the transformation of the github repository to the output of /repos and /stats
* add some tests on the filtering
* a Makefile with rules to execute the tests, execute golangci-lint
* a github action that would call thoses Makefile rules on push
* some clean commits

That was a fun test, thank you !
