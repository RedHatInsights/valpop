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

Global Flags:
  -h, --help              help for valpop
  -a, --hostname string   Valkey hostname (default "127.0.0.1")
  -p, --port string       Valkey port (default "6379")
  -m, --mode string       Mode, s3 or valkey (default "s3")
  -u, --username string   Username for S3
  -c, --password string   Password for S3
  -b, --bucket string     S3 bucket name (default "frontend")

Use "valpop [command] --help" for more information about a command.
```

## Commands

### populate
Populates the cache from the source directory.

**Usage:**
```
valpop populate [flags]
```

**Flags:**
```
  -s, --source string     Source directory (required)
  -r, --prefix string     Prefix for dir structure and cache (required)
  -t, --timeout int       Timeout for cache (default 30)
```

**Example:**
```bash
valpop populate --source /path/to/assets --prefix myapp --timeout 60
```

### pop
Copies cached files to the destination directory for serving.

**Usage:**
```
valpop pop [flags]
```

**Flags:**
```
  -d, --dest string       Destination directory (required)
```

**Example:**
```bash
valpop pop --dest /var/www/html
```

### Environment Variables
All flags can also be set using environment variables with the `VALPOP_` prefix:

- `VALPOP_HOSTNAME` - Valkey hostname
- `VALPOP_PORT` - Valkey port
- `VALPOP_MODE` - Mode (s3 or valkey)
- `VALPOP_USERNAME` - S3 username
- `VALPOP_PASSWORD` - S3 password
- `VALPOP_BUCKET` - S3 bucket name
- `VALPOP_SOURCE` - Source directory
- `VALPOP_PREFIX` - Prefix for cache keys
- `VALPOP_TIMEOUT` - Cache timeout
- `VALPOP_DEST` - Destination directory

# Running with Podman
```
$ podman build -f Dockerfile -t valpop .
$ podman run -it valpop {args}
```
