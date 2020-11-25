# Air Alert

Air Alert is a web application that provides air quality forecasting and 
notifications. It takes data from [Purple Air](https://www2.purpleair.com/) and
processes it to create local future projections of air quality and provides 
that data to users in the form of web notification and through an API for more
advanced users.

You can view the live site at [airalert.app](https://airalert.app).

## Usage
### Prerequisites
The recommended method for running this application yourself is to use Docker
Compose. You will need to have Docker and Docker Compose installed before 
continuing.

### Initial Setup
You will need to configure the application before you can use it. There is a
utility built into the executable that initializes a configuration file with
some sane defaults. In order to initialize the configuration file, just run the
following commands.

```bash
$ touch config.toml
$ docker run \
  -v $(pwd)/config.toml:/config.toml:rw \
  mrflynn/air-alert:latest init-config
```

This will populate the configuration file with all default configuration values.
You can leave most of these as-is.

If you wish to change certain settings or get a better understanding of what 
they do, head down to the [Configuration](#configuration) section of this 
document for more information. If you are serving this application publicly,
I recommend enabling SSL. See the [SSL](#web.ssl) section for more information.

Finally, you will need to generate two passwords: one for the postgres root
account and one for the postgres storage account. Set the following 
environment variables with their respective values: `AIR_ALERT_PGPASS` and
`POSTGRES_PASSWORD`.

### Running the Application
Running the application with Docker Compose is quite easy. Just run

```bash
$ mkdir -p data/postgres data/redis
$ docker-compose up -d
```

and the application will launch. It should be available at port 3000 now.

## Configuration
This section details how to configure Air Alert. Below you can find a 
recommended configuration and details on all options available to you.

### Recommended Configuration
This is a recommended configuration file before running `init-config`. Disable
SSL if you don't need it.

```toml
timezone = "America/Los_Angeles" # Use your local time zone, or UTC.

[database]

  [database.postgres]
    database = "airalert"
    host = "localhost"
    password = "password" # Use something more secure.
    port = 5432
    username = "airalert"

  [database.redis]
    addr = ":6379"
    id = 0
    password = "password" # Use something more secure.

[web]

  [web.notifications]
    admin_mail = "admin@example.com" # Use your actual email.

  [web.ssl]
    domains = ["airalert.app", "www.airalert.app"]
    email = "admin@airalert.app"
    enable = true

```

Also, ensure that if you do configure a password for Redis or Postgres then
this is also reflected in the configuration of each, respectively. In the default
configuration created when you run Air Alert under Docker Compose, Redis won't
have a password so this field should be left blank. You can alter compose to
add a password, but that is not the default behavior.

### Options
* **timezone**: Application timezone. This configures the timezone under which
time-based background tasks are configured to run. Must be a valid timezone 
identifier. Defaults to UTC.

#### `database.postgres`
This section configures the program's access to the Postgres database. This 
database stores notification preferences. It is recommended that you have your 
own username + password set.

* **host**: Hostname or IP address of the Postgres server. Default is 
`localhost`.
* **port**: Port on the database server that Postgres is running on. Default is 5432.
* **database**: Postgres database to use. Default is "airalert".
* **username**: Username of Postgres user with access to **database**. Default
is "postgres".
* **password**: Password of Postgres user. Default is empty string.
* **ssl_mode**: Postgres SSL configuration. Default is "require". Valid options
include "disable", "require", "verify-ca", and "verify-full". See 
[lib/pq](https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters)
sslmode settings for a more in-depth description for each option.

#### `database.redis`
This section configures access to the Redis datastore which is used to store
sensor data, locations, and for stream processing storage. It is recommended 
that you set your own password.

* **addr**: Address and port of the Redis datastore. Default is `0.0.0.0:6379`.
* **id**: ID of the database within the Redis instance. Default is 0.
* **password**: Password for the instance. Default is empty string.

#### `purpleair`
This configures access to Purple Air's API. These options are only for 
debugging purposes and should not be changed.

* **url**: URL of the Purple Air API. Default is "https://purpleair.com/json".
* **rate_limit_timeout**: How often the program can issue a request to Purple
Air's API. Default is 10 seconds.

#### `web`
This section configures general web server options. These should be kept at 
their defaults in most cases.

* **addr**: Address and port of web server. Default is `0.0.0.0:3000`.
* **static_dir**: Directory containing static CSS and Javascript files.
Default is `./static` <sup>\[1\]</sup>.
* **template_dir**: Directory contain server-side rendered HTML templates. 
Default is `./templates`.

#### `web.notifications`.
These options configure the web notification settings. Most of these can be
left alone. When you run `init-config`, the VAPID keys required for web 
notifications will be automatically generates for you.

* **admin_mail**: Administrative email used for web notifications. Default is
"admin@localhost".
* **group**: Redis stream group for notification queue storage. Default is
"notification_delivery".
* **private_key**: VAPID private key. Default is empty string<sup>\[2\]</sup>.
* **public_key**: VAPID public key. Default is empty string<sup>\[2\]</sup>.
* **threads**: Number of consumer threads for the notification queue. Default 
is 4.

#### `web.ssl`
These options are used to configure SSL for the web server. If enabled, you
need to provide a list of domains that the server wants to use SSL with. It is
recommended if you give a domain, you provide the same domain with "www." 
prepended to it.

* **enable**: Enable SSL. Default is false.
* **domains**: List of domains to serve SSL for. Default is empty list of 
strings<sup>\[3\]</sup>.
* **email**: Email to get Let's Encrypt renewal notifications. Default is empty
string.

### Caveats
* \[1\]: If you are not user Docker Compose and you downloaed the tar archive 
from Github, then you will need to change this option to `./static/dist`.
* \[2\]: These are automatically filled in when you run the `init-config` 
subcommand.
* \[3\]: If SSL is enabled and no domains are provided, then the program will
not start up.

### Full Configuration File
Here is an example of a complete configuration file with default values. Refer 
to any section above for a more in-depth description of each option.

```toml
timezone = "UTC"

[database]

  [database.postgres]
    database = "airalert"
    host = "localhost"
    password = ""
    port = 5432
    username = "postgres"
    ssl_mode = "require"

  [database.redis]
    addr = ":6379"
    id = 0
    password = ""

[purpleair]
  rate_limit_timeout = "10s"
  url = "https://www.purpleair.com/json"

[web]
  addr = ":3000"
  static_dir = "./static"
  template_dir = "./templates"

  [web.notifications]
    admin_mail = "admin@localhost"
    group = "notification_delivery"
    private_key = "<private_key>"
    public_key = "<public_key>"
    threads = 4

  [web.ssl]
    domains = [""]
    email = ""
    enable = false
```

## Contributing
Contributions are tentatively welcome. This is a personal project for me so
I don't know if I will accept any outside help for the moment. Feel free to
make or propose changes and we can have a discussion about them.

## Contributors
* [Nick Pleatsikas](https://github.com/MrFlynn)

## License
[AGPL-3.0](LICENSE)