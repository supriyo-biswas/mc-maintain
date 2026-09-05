# mc

`mc` provides a command-line client to interact with Amazon S3 and S3-compatible object storage services (e.g. Cloudflare R2, Backblaze B2, MinIO, etc.) It offers a Unix-like command based interface to interact with S3, implementing commands like ls, cat, cp, find etc.

It provides the following commands:

```
  alias      manage server credentials in configuration file
  anonymous  manage anonymous access to buckets and objects
  cp         copy objects
  cat        display object contents
  diff       list differences in object name, size, and date between two buckets
  du         summarize disk usage recursively
  encrypt    manage bucket encryption config
  event      manage object notifications
  find       search for objects
  get        get s3 object to local
  head       display first 'n' lines of an object
  ilm        manage bucket lifecycle
  legalhold  manage legal hold for object(s)
  ls         list buckets and objects
  mb         make a bucket
  mv         move objects
  mirror     synchronize object(s) to a remote site
  pipe       stream STDIN to an object
  put        upload an object to a bucket
  rm         remove object(s)
  retention  set retention for object(s)
  rb         remove a bucket
  sql        run sql queries on objects
  stat       show object metadata
  share      generate URL for temporary access to an object
  tree       list buckets and objects in a tree format
  tag        manage tags for bucket and object(s)
  undo       undo PUT/DELETE operations
  version    manage bucket versioning
  watch      listen for object notification events
```

It is based on the last open-source version of the [Minio client](https://github.com/minio/mc), but with some differences, see [compatibility](#compatibility).

## Installation

To install the mc client, head to the [releases](https://github.com/supriyo-biswas/mc/releases) and download a binary for the platform of your choice.

On Linux/MacOS/FreeBSD, you can install the `mc` binary to `/usr/local/bin` using the following command:

```
curl -fSL https://github.com/supriyo-biswas/mc/releases/latest/download/mc-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed -e 's/^x86_64$/amd64/' -e 's/^aarch64$/arm64/' -e 's/^armv.*$/arm/') -o mc && chmod +x mc && sudo mv mc /usr/local/bin/
```

## Usage

To begin, start by adding an object storage service to `mc`'s configuration by using the `mc` alias set command.

Storage operations require a configured alias. Local filesystem paths are accepted only when transferring data to or from object storage; local-only operations are rejected to avoid treating a missing alias as a local path.

You'll need an access key and secret to get started. For example, if you want to access Amazon S3, create an [access key in IAM](https://medium.com/@anuradha.kadurugasyaya/create-aws-iam-user-for-s3-bucket-892bae4751fc) and add it using the following command:

```bash
mc alias set s3 https://s3.amazonaws.com <access-key> <secret-key> --path dns --api S3v4
```

Similarly, for Google Cloud Storage, you can create an [access key in Google Cloud](https://documentation.arcserve.com/Arcserve-Cloud/Available/Cloud_Console/ENU/olh/Cloud%20Console/creating_access_secret_keys_gc.htm) and add it using the following command:

```bash
mc alias set gcs https://storage.googleapis.com <access-key> <secret-key> --path dns --api S3v2
```

If you have neither, you test against the [MinIO Play server](https://play.min.io) by adding it as an alias:

```bash
mc alias set play https://play.min.io Q3AM3UQ867SPQQA43P2F zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG
```

Once you've done this, you can use the `mc` commands to manage your object storage, like so (replace `s3` with `gcs` or `play` or another alias that you've set):

```bash
# list your buckets
mc ls s3
# create a bucket
mc mb s3/my-bucket
# copy a file to a bucket
mc cp my-file.txt s3/my-bucket
# list the contents of a bucket
mc ls s3/my-bucket
```

### Shell autocompletions

In case you are using bash, zsh or fish. Shell completion is embedded by default in `mc`, to install auto-completion use `mc --autocompletion`. Restart the shell, mc will auto-complete commands as shown below.

```
mc <TAB>
config   diff     find     ls       mirror   policy   session  sql      watch
cat      cp       event    head     mb       pipe     rm       share    stat     version
```

## Compatibility

`mc` aims to be compatible with Amazon S3 and S3-compatible services. Every commit is tested in CI against the following self-hosted S3-compatible services:

* [RustFS](https://github.com/rustfs/rustfs)
* [Garage](https://garagehq.deuxfleurs.fr)
* [SeaweedFS](https://github.com/seaweedfs/seaweedfs)
* [VersityGW](https://github.com/versity/versitygw)
* [MinIO Aistor](https://www.min.io/product/aistor)

If you've faced compatibility issues with S3 compatible services please [open an issue](https://github.com/supriyo-biswas/mc/issues).

There are a few differences in the CLI commands as well:

* MinIO-only commands such as `admin`, `support` and `license` are not included as we target broader S3 compatibility instead of being an administration tool for MinIO.
* Local-only operations such as listing local files or deleting local files are not supported, as the author has found it only leads to mistakes such as inadvertently deleting local files. Local↔S3 and S3↔S3 transfers and operations are supported.

## License

`mc` is licensed under  the GNU AGPLv3 license that can be found in the [LICENSE](https://github.com/supriyo-biswas/mc/blob/master/LICENSE) file, which is the same license as the original [MinIO client](https://github.com/minio/mc).
