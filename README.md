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

It is based on the last open-source version of the [Minio client](https://github.com/minio/mc), but some commands are unavailable, see [compatibility](#compatibility).

## Installation

To install the minio client, head to the [releases](https://github.com/supriyo-biswas/mc-maintain/releases) and download a binary for the platform of your choice.

On Linux/MacOS/FreeBSD, you can install the `mc` binary to `/usr/local/bin` using the following command:

```
curl -fSL https://github.com/supriyo-biswas/mc-maintain/releases/latest/download/mc-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed -e 's/^x86_64$/amd64/' -e 's/^aarch64$/arm64/' -e 's/^armv.*$/arm/') -o mc && chmod +x mc && sudo mv mc /usr/local/bin/
```

## Add a Cloud Storage Service
If you are planning to use `mc` only on POSIX compatible filesystems, you may skip this step and proceed to [everyday use](#everyday-use).

To add one or more Amazon S3 compatible hosts, please follow the instructions below. `mc` stores all its configuration information in ``~/.mc/config.json`` file.

```
mc alias set <ALIAS> <YOUR-S3-ENDPOINT> <YOUR-ACCESS-KEY> <YOUR-SECRET-KEY> --api <API-SIGNATURE> --path <BUCKET-LOOKUP-TYPE>
```

`<ALIAS>` is simply a short name to your cloud storage service. S3 end-point, access and secret keys are supplied by your cloud storage provider. API signature is an optional argument. By default, it is set to "S3v4".

Path is an optional argument. It is used to indicate whether dns or path style url requests are supported by the server. It accepts "on", "off" as valid values to enable/disable path style requests.. By default, it is set to "auto" and SDK automatically determines the type of url lookup to use.

### Example - MinIO Cloud Storage
MinIO server startup banner displays URL, access and secret keys.

```
mc alias set minio http://192.168.1.51 BKIKJAA5BMMU2RHO6IBB V7f1CwQqAcwo80UEIJEjc5gVQUSSx5ohQ9GSrr12
```

### Example - Amazon S3 Cloud Storage
Get your AccessKeyID and SecretAccessKey by following [AWS Credentials Guide](http://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html).

```
mc alias set s3 https://s3.amazonaws.com BKIKJAA5BMMU2RHO6IBB V7f1CwQqAcwo80UEIJEjc5gVQUSSx5ohQ9GSrr12
```

**Note**: As an IAM user on Amazon S3 you need to make sure the user has full access to the buckets or set the following restricted policy for your IAM user

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowBucketStat",
            "Effect": "Allow",
            "Action": [
                "s3:HeadBucket"
            ],
            "Resource": "*"
        },
        {
            "Sid": "AllowThisBucketOnly",
            "Effect": "Allow",
            "Action": "s3:*",
            "Resource": [
                "arn:aws:s3:::<your-restricted-bucket>/*",
                "arn:aws:s3:::<your-restricted-bucket>"
            ]
        }
    ]
}
```

### Example - Google Cloud Storage
Get your AccessKeyID and SecretAccessKey by following [Google Credentials Guide](https://cloud.google.com/storage/docs/migrating?hl=en#keys)

```
mc alias set gcs  https://storage.googleapis.com BKIKJAA5BMMU2RHO6IBB V8f1CwQqAcwo80UEIJEjc5gVQUSSx5ohQ9GSrr12
```

## Test Your Setup
`mc` is pre-configured with https://play.min.io, aliased as "play". It is a hosted MinIO server for testing and development purpose.  To test Amazon S3, simply replace "play" with "s3" or the alias you used at the time of setup.

*Example:*

List all buckets from https://play.min.io

```
mc ls play
[2016-03-22 19:47:48 PDT]     0B my-bucketname/
[2016-03-22 22:01:07 PDT]     0B mytestbucket/
[2016-03-22 20:04:39 PDT]     0B mybucketname/
[2016-01-28 17:23:11 PST]     0B newbucket/
[2016-03-20 09:08:36 PDT]     0B s3git-test/
```

Make a bucket
`mb` command creates a new bucket.

*Example:*
```
mc mb play/mybucket
Bucket created successfully `play/mybucket`.
```

Copy Objects
`cp` command copies data from one or more sources to a target.

*Example:*
```
mc cp myobject.txt play/mybucket
myobject.txt:    14 B / 14 B  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  100.00 % 41 B/s 0
```

## Everyday Use

### Shell autocompletion
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

If you've faced compatibility issues with S3 compatible services please [open an issue](https://github.com/supriyo-biswas/mc-maintain/issues).

MinIO-only commands such as `admin`, `support` and `license` are not included as we are targeting broader S3 compatibility instead of being an administration tool for MinIO.

## License

Use of `mc` is governed by the GNU AGPLv3 license that can be found in the [LICENSE](https://github.com/supriyo-biswas/mc-maintain/blob/master/LICENSE) file, which is the same license as the original [MinIO client](https://github.com/minio/mc).
