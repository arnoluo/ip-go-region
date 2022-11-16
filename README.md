# ip go region

本项目基于[ip2region](https://github.com/lionsoul2014/ip2region)，调整其golang生成及查询功能代码，旨在增加较少io的情况下缩减xdb大小。

## 调整
### 拆分地域内容存储格式
地域头部信息，尾部信息分开存储，减小存储重复，存储结构相同，如下：
|strLen|str|
|-|-|
|1 Byte|n Bytes|

第一个Byte统一用来存储后面的字符长度
头部信息基本固定，所以限制长度<=64B，即 n <= 63
尾部信息可扩展，可以达到1B能代表的最大长度，即 n <= 255

由于目前基础信息都是使用[ip.merge.txt](https://github.com/lionsoul2014/ip2region/blob/master/data/ip.merge.txt)生成，所以拆分时也使用此格式。

`startIp|endIp|国家|区域|省(州)|城市|isp`

#### 地域头部信息
包含前三段：`国家|区域|省(州)`，限制63B内, 固定增加一次io，全部信息排重记录后，记录整个头部区块的开始地址到header内，单条信息的地址由相对于开始地址的偏移量确定，固定2B，后面存入到二分索引区块内，替代原来块内的长度标志段

#### 地域尾部信息
即为前三段后所有内容，目前包括 `城市|isp`，可追加，
63B内则同原查询一样仅一次io，超过后(<255B)将增加一次io。

### 缩减二分索引长度，由原来的14Bytes 改为 10Bytes，新结构为
|startIpTail|endIpTail|regionHeadOffset|regionTailPtr|
|-|-|-|-|
|2B|2B|2B|4B|
> 由于所有查询均使用了vector索引，所以二分区块去除了无用的ip前二段数值，仅保留开始与结束ip的后两段，即startIpTail,endIpTail