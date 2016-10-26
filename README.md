# superserver
Simple superserver program inspired by inetd and xinetd.
Listens to new connections on TCP port and starts service program with stdin, stdout and stderr set to connection socket.


##Command-line options
```
-e string
        Program to be executed
-p int
        Port number to listen to (default -1)
-t duration
        Child services termination timeout (default 3s)
```
 
###__SIGINT__, __SIGTERM__
Makes superserver stop accepting new connections, send ```SIGTERM``` to child services and wait timeout for them to complete before
killing child services and exiting.

##Details
Child services has empty environment.
Killing child services doesn't kills processes started by them: superserver only kills its children, but not grandchildren.

##TODO
inetd/xinetd-like config and command-line options.