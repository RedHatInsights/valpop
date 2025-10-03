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
After population, Valpop will clean up old cache entries based on two criteria:
1. **Timeout**: Entries older than the specified timeout will be eligible for cleanup
2. **Minimum Asset Records**: At least the specified minimum number of asset versions will always be preserved, regardless of timeout

This ensures that even if all cached versions exceed the timeout, the most recent versions are still retained for availability.

Future versions of Valpop may do deduplication for efficient cache usage.

## Pop
Valpop will pull down all the files in the cache and put them into the `dest`
directory. It will apply them in the order that they were added to the cache,
using the timestamp as part of the key. This means that even if an older build
were to be run up, those files WOULD NOT become the latest version.

If the desired behaviour were to rollback, a revert commit should be used to
build a new version of the image, OR the cache would need purging manually of
that content.


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
  -s, --source string           Source directory (required)
  -r, --prefix string           Prefix for dir structure and cache (required)
  -t, --timeout int             Timeout for cache cleanup in seconds (default 30)
  -n, --min-asset-records int   Minimum number of asset records to keep (default 3)
```

**Examples:**
```bash
# Basic usage with default minimum asset records (3)
valpop populate --source /path/to/assets --prefix myapp --timeout 60

# Keep at least 5 versions of each asset, even if they exceed timeout
valpop populate --source /path/to/assets --prefix myapp --timeout 60 --min-asset-records 5

# Using short flags
valpop populate -s /path/to/assets -r myapp -t 60 -n 5
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

## Cache Cleanup Behavior

When running `populate`, Valpop performs intelligent cache cleanup based on two parameters:

### Timeout (`--timeout` / `-t`)
- **Default**: 30 seconds
- **Purpose**: Defines how old an asset can be before it becomes eligible for cleanup
- **Behavior**: Assets older than this threshold will be considered for removal

### Minimum Asset Records (`--min-asset-records` / `-n`)
- **Default**: 3
- **Purpose**: Ensures a minimum number of asset versions are always preserved
- **Behavior**: At least this many versions of each asset will be kept, regardless of age

### Cleanup Logic
The cleanup process follows this priority:
1. **Always preserve** at least `min-asset-records` versions of each asset (newest first)
2. **Only delete** versions that are both:
   - Beyond the minimum count requirement AND
   - Older than the `timeout` threshold

### Examples
```bash
# Scenario 1: Only keep 1 version, clean up after 5 minutes
valpop populate -s ./assets -r myapp -t 300 -n 1

# Scenario 2: Keep 10 versions, clean up after 24 hours
valpop populate -s ./assets -r myapp -t 86400 -n 10
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
- `VALPOP_TIMEOUT` - Cache timeout in seconds
- `VALPOP_MIN_ASSET_RECORDS` - Minimum number of asset records to keep
- `VALPOP_DEST` - Destination directory

# Building with Podman
```bash
podman build -f Dockerfile -t valpop .
podman run -it valpop {args}
```

# Testing locally

You will need to have `podman` installed to test the interaction between valkey or minio (s3)

To build the valpop cli, run:

```bash
make build
```

### Testing with minio (s3)

1. Start the minio server by running:
   ```bash
   make start-minio
   ```
2. From a different tab in your terminal, run the following command to create the necessary bucket:
   ```bash
   make create-minio-buckets
   ```
3. Upload to minio using valpop by running:
   ```bash
   make minio-test-upload
   ```
4. To verify, log into the minio WebUI (url should be displayed to you after completing step 1, but this is usually: http://127.0.0.1:10000). The username and password (MINIO_ACCESS_KEY/MINIO_SECRET_KEY) for the minio console are in the `.env` file.


### Testing with valkey

1. Start the valkey server by running:
   ```bash
   make start-valkey
   ```
2. Upload to valkey using the valpop cli by running:
   ```bash
   make valkey-test-upload
   ```
3. To verify your upload, you will need to first start the valkey cli by running:
   ```bash
   make start-valkey-cli
   ```
4. Then inside the CLI, run the command:
   ```bash
   keys *
   ```
