# superserver
Simple superserver program inspired by inetd and xinetd.
Listens to new connections on TCP port then starts appropriate service program (specified in config) reading from connection socket to service's stdin
and writes to socket from service's stdout.

## Command-line Options
```
-f string
    Config file path (default is superserver.toml)
-t duration
    Child service termination timeout (default 3s)
-limit uint (default 0)
    Maximum number of concurrently running processes that can be started. 0 for unlimited.
```

## Config File Syntax
```
[[service]]
name = "very-test" # unique name to help identify service
port = 3030 # service port
program = "/home/me/service" # program to be executed for service
program-args = ["service", "foo"] # arguments to be passed for service program, optional

[[service]]
...
```

## __SIGINT__, __SIGTERM__
Makes superserver stop accepting new connections, closes children stdin and if they
didn't exit — send them ```SIGTERM``` and wait termination timeout for services to complete before
killing them and exiting superserver.

## Details
Writing to child service's stderr sends data to superserver's stderr.

Child services have empty environment.

Killing child services doesn't kill processes started by them: superserver only kills its children, but not grandchildren.

There's a known race condition when using service's IO streams and calling `cmd.Wait` — [os/exec: data race between StdinPipe and Wait](https://github.com/golang/go/issues/9307).

## TODO
* Better logging.
* More inetd/xinetd-like config and command-line options.
* Check (and maybe allow to configure) read buffer size for network connections and child services.
