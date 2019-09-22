# Synca

Continuously replicates a filesystem to cloud storage. At the moment the command only accepts a single directory. This project is mostly an experiment at this stage and has lots of bugs - do not use in production!

```
./synca /data mybucketname eu-west-1
```

Synca supports uploading to multiple cloud providers, but only AWS is implemented at the moment.

## How it works

1. Filesystem events are monitored using the inotify API
2. Information about these events is sent to a central queue
3. The queue manager replicates these events to _n_ provider-specific queues
4. The cloud provider package (e.g. `clouds/aws`) is responsible for managing its own queue and replicating the filesystem events in the cloud storage.

## TODO

- Use a proper library for parsing cli options
- Allow use of a configuration file
- Check and handle errors better
