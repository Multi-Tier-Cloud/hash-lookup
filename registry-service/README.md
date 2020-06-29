# registry-service

Note regarding restarting etcd cluster:
If you kill majority of etcd nodes, ie. you lose quorum, you will need to restore the cluster.
Trying to simply restart any of the nodes doesn't work.
Etcd retains its old ID and cluster membership info in its data directory (`<name>.etcd`).
Restarting a node with its old data directory causes it to attempt and fail to reach quorum 
among the old members, which might not even be alive anymore.
Starting a brand new cluster means you lose key-value store data.
Trying to connect a new node to a node from the old cluster causes mismatched cluster IDs.

In my own testing, this is how I recovered the key-value store while starting a new cluster.

1. Kill all etcd and hl-service nodes.

2. Pick one node. Save its snapshot "db" file somewhere. It can be found under etcd's data 
directory, `<name>.etcd/member/snap/db`. You will then have to delete (or just move to be safe)
the data directory.
Note in hl-service the convention for name is `<ip>-<client_port>-<peer_port>` eg. `10.11.17.11-2379-2380`

3. Restore from the snapshot db, creating a new data directory, but with the ID and cluster
membership info overwritten, allowing you to start a new cluster, but with the old key-value data.
```
$ etcdctl snapshot restore <path/to/snapshot/db> \
  --name <name> \
  --initial-cluster <name>=http://<ip>:<peer_port> \
  --initial-advertise-peer-urls http://<ip>:<peer_port> \
  --skip-hash-check=true
```

eg.
```
$ etcdctl snapshot restore old.etcd/member/snap/db \
  --name 10.11.17.11-2379-2380 \
  --initial-cluster 10.11.17.11-2379-2380=http://10.11.17.11:2380 \
  --initial-advertise-peer-urls http://10.11.17.11:2380 \
  --skip-hash-check=true
```

4. Start hl-service as if you were starting a new cluster. You should find you can successfully
run this node and still have the original key-value data.
eg.
`$ ./hl-service --etcd-ip 10.11.17.11 --new-etcd-cluster`

5. To restart other nodes, delete the other nodes' etcd data directory and run hl-service as if you
were connecting to an existing cluster.
eg.
`$ ./hl-service --etcd-ip 10.11.17.13`

6. Done.

More info: https://etcd.io/docs/v3.3.12/op-guide/recovery/