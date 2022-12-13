

# Start CubeFS Cluster

* Start CubeFS  cluster at first, refer: [CubeFS deploy](https://cubefs.readthedocs.io/en/latest/manual-deploy.html)
* Create Storage volume, volume name style is, volume_$number, eg: volume_1

```plain
 curl -v "http://10.196.59.198:17010/admin/createVol?name=volume_1&capacity=10000&owner=cfa"
```

# Build CFA Server

* **Repo**：[CubeFS-code](https://github.com/cubefs/cubefs.git)
* **Branch**：cubefs-for-android-server
* **Build cfa-server**
    * Set gopath, **cd** **$GOPATH/src/github.com/cubefs/cubefs/cfa**
    * Execute **source build.sh** to build **cfa-server**
# Start Proxy

* Introducation
    * Support http api for cfa-client to invoke.
    * Support third auth(optional), user route(optional).
* Start Process

```plain
./cfa-server -c proxy.conf
```

* Proxy.conf Sample:

```plain
{
    "role":"proxy",
    "port":"19010",
    "logLevel":"debug",
    "master_addr":"10.177.69.105:17010,10.177.69.106:17010,10.177.117.108:17010",
    "volName":"volume_1",
    "logDir":"/cfs/logs"
}
```

* Configuration

|**Name**|**Type**|**Description**|**Mandatory**|
|:----|:----|:----|:----|
|role|string|Role of process and must be set to **proxy**|Yes|
|port|string|Listen and serve port|Yes|
|master_addr|string| Address of master servers|Yes|
|logDir|string|Log directory|Yes|
|logLevel|string|Log level, default **error**|No|
|routerAddr|string|Router server addr and port,eg: `http://127.0.0.1:18010`|No|
|volName|string|Volume name, used to store different users's data, eg: volume_1|No|
|authAddr|string|Third auth url, eg: `http://127.0.0.1:10121/third/token/check`|No|
|authAccessKey|string|Third auth key|No|

There must be at least one of RouterAddr or volName, **if both exist, then volName will take effect**

- **volName**: if used, proxy module will manage users router, cluster only support one volume.

* **routerAddr**：if used, user's relationship will be managed by route server, cluster can support multiple volumes.

* Create User Sapce
Before using it, the user needs to call the following interface to open a space for the user, where userId is the user account of each device.

```plain
curl proxy:19010/api/v1/createUser -X POST -H "x-userId: 1234567" -i
```

The directory of user 1234567 in CFA will eventually be mapped to the /$hashId/1234567/0/ directory on the volume (such as volume_1).

# Start Router (optional)

* Introducation
    * Used to alloc storage volume and subpath for users.
    * Used to persistently store the mapping relationship between users and volumes.
* Dependency
    * [mongodb](https://www.mongodb.com/docs/manual/tutorial/)
* Start Process

```plain
./cfa-server -c router.conf
```

* Router.conf Sample

```plain
{
    "role":"route",
    "port":"18010",
    "logLevel":"debug",
    "mgoAddr":"mongodb://user:password@host1:20012,host2:20010/dbname",
    "logDir":"/cfs/logs"
}
```

* Configuration

|**Name**|**Type**|**Description**|**Mandatory**|
|:----|:----|:----|:----|
|role|string|Role of process and must be set to **route**|Yes|
|port|string|Listen and server port|Yes|
|master_addr|string|Address of master servers|Yes|
|logDir|string|Log directory|Yes|
|logLevel|string|Log level, default error|No|
|mgoAddr|string|Mongo addr|Yes|

* Init Route
When using the routing service, you need to call the interface to pre-add the created volume to the routing table firstly. When users are created, users will be allocated to these volumes according to the allocation algorithm; voumeid is the suffix number of the volume name created in the cubefs cluster. For example, volume name vol_1, volumeId=1; volume name vol_10, volumeid=10

```plain
curl 127.0.0.1:18010/router/api/v1/addvol -d '{"volumeid":2}' -X POST
```

# Start Third Auth Service (optional)

* Introducation
    * Provide authentication service for proxy module, implemented by third-party developers with reference to the following specifications: [auth-doc](./third-auth.md)
* Third-auth-example(demo)
    * cd $GOPATH/src/github.com/cubefs/cubefs/proxy/third-auth-example
    * go build
* Start Third-auth-example

```plain
./third-auth-example 
```

- add config in proxy's config file

```plain
"authAddr":"http://localhost:10121/third/token/check",
"authAccessKey": "accessKey",
```
    
