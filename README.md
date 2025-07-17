# Valpop
Valpop is a cache populator and popper, to be used with the ConsoleDot Frontend system.

# Design
## Populate
Valpop is designed to be run inside a Frontend container to take all the assets
and to dump them into a ValKey/Redis cache. Each entry in the cache will have
a key structure that looks like this: `prefix:timestamp:some/path/to/filename`.

The `prefix` is intended to be the application name. The `source` is a directory
where the root of the files lives.

The `timestamp` should be the build timestamp of the container and is important.
After population, Valpop will clean up all files which are older than the `timestamp` specified, but will always leave the latest version. If files are
newer than the timestamp then multiple version will be kept. Not setting `timestamp`
will yield many versions of the same file landing in the cache.

Future versions of Valpop may do deduplication for efficient cache usage.

## Pop
Valpop will pull down all the files in the cache and put them into the `dest`
directory. It will apply them in the order that they were added to the cache,
using the timestamp as part of the key. This means that even if an older build
were to be run up, those files WOULD NOT become the latest version.

If the desired behaviour were to rollback, a revert commit should be used to
build a new version of the image, OR the cache would need purging manually of
that content.

# Running locally
A ValKey instance is required. Podman can help.
```
docker run --replace --name some-valkey --network host -d valkey/valkey
docker run -it --network host --rm valkey/valkey valkey-cli -h 127.0.0.1
```

# Usage
```
pops or populates Valkey for Frontends - ya know

Usage:
  valpop [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  pop         copies to the dest for serving
  populate    populates the cache

Flags:
  -h, --help              help for valpop
  -a, --hostname string   Valkey hostname (default "127.0.0.1")
  -p, --port string       Valkey port (default "6379")

Use "valpop [command] --help" for more information about a command.
```

# Running with Podman
```
$ podman build -f Dockerfile -t valpop .
$ podman run -it valpop {args}
```

Leaf
