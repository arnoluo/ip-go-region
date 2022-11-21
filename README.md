# ip go region

一个提供快速ip到区域转换的离线库
本项目基于 [ip2region v2](https://github.com/lionsoul2014/ip2region)，分离出 `golang` 版本功能代码并作结构调整，旨在增加少量io的情况下缩减xdb大小。

## 进度
|主要指标|本项目|原项目|
|-|-|-|
|xdb文件大小|6.5M|11.1M|
|单次查询io|num + 1|num|

> 本项目xdb所有配置下查询io固定为 num + 1，若有对原始文件 `ip.merge.txt` 行内容进行扩充并重新编译xdb文件，可能存在 `地域尾部信息` 超长，此时io变为 num + 2

## 使用
### 安装
`go install github.com/arnoluo/ip-go-region`

### 查询
```golang
package main

import (
	"github.com/arnoluo/ip-go-region/xdb"
)

func main() {
    // 默认xdb文件，位于本项目`/data/igr.xdb`，可复制或自定义编译xdb文件后使用
    yourXdbPath := "your/path/to/igr.xdb"

    // 查询器初始化时的三种缓存方式，按实际场景选其中之一使用
    // 完全基于文件io查询
	// fileSe, err := xdb.Create(yourXdbPath, xdb.CACHE_POLICY_FILE)
    // 或 vector内存索引加速查询
    // vectorSe, err := xdb.Create(yourXdbPath, xdb.CACHE_POLICY_VECTOR)
    // 或 完全内存查询
    // memorySe, err := xdb.Create(yourXdbPath, xdb.CACHE_POLICY_MEMORY)

    // 以完全内存查询为例
    memorySe, err := xdb.Create(yourXdbPath, xdb.CACHE_POLICY_MEMORY)
    if err != nil {
        panic(err)
    }

    /////////// 以下为配置项，可跳过 /////////////
    // 设置是否开启完全查询，默认开启，此选项仅影响返回的地域尾部信息长度
    // 开启时，将获取完整地域尾部信息，若原始文件有自定义扩充，可能导致查询io 为 num + 2
    // 关闭时，将仅截取默认长度的地域尾部信息，默认长度由SetMatchTailLen()决定，此操作将固定查询io为 num+1
    memorySe.SetSearchMode(false)

    // 设置地域尾部信息默认查询长度，默认64（Bytes）
    // 此选项若设置为最大值255，所有查询io将均为 num+1，此时SetSearchMode()不再影响查询结果
    // 此选项仅建议在优化原始文件`地域尾部信息`长度后进行调整
    memorySe.SetMatchTailLen(128)
    /////////// 以上为配置项，可跳过 /////////////

    // 获取地域信息
    regionStr, err := memorySe.SearchByStr("2.12.133.0")
    // 获取地域信息后，可查看查询io情况
    ioCount := memorySe.GetIOCount()
}
```

### 编译
编译前请先下载[ip.merge.txt](https://github.com/lionsoul2014/ip2region/blob/master/data/ip.merge.txt)，或按照行格式自行创建原始文件，格式为 `startIP|endIP|国家|区域|省(州)|城市|isp` + `任意扩展字符串`。然后将原始文件放入 `data` 目录(此为默认编译目录，可编辑 `Makefile` 进行修改)，即可进行编译。
若为自行创建的原始文件，或对原始文件进行扩充，建议先阅读[拆分地域信息](###拆分地域信息)部分，了解各段信息的填充限制，避免编译失败。
编译后使用 `make bench` 完成全ip覆盖测试后方可使用

```bash
# 生成编译器
make

# 生成xdb文件，请做好备份
make gen

# bench test
make bench

# 开启search console
make search

# 移除编译器
make clean
```

### 其他说明
本文档主要对调整部分进行了说明，若有其他方面疑问，可参考[原项目文档](https://github.com/lionsoul2014/ip2region)


## 结构调整
### 拆分地域信息
由于目前地域信息都是使用 `ip.merge.txt` 进行生成，所以本次拆分也基于此文件行格式，即：
`startIP|endIP|国家|区域|省(州)|城市|isp`

其中地域信息 `国家|区域|省(州)|城市|isp` 拆分为头部，尾部两段分开存储，以减小重复内容


#### 1. 地域头部信息
包含前三段：`国家|区域|省(州)`，头部信息相对固定，排重后优先全部存储并获取块起始地址，目前总数为40397B，采用块起始地址(4B)+偏移量(2B)进行定位，块起始地址存储于header[16:20]，偏移量则记入到 `地域尾部信息` 行内标志位，此调整将固定增加一次io。

头部信息行存储结构：
|信息长度|信息|
|-|-|
|1 Byte|n Bytes(n < 64)|

> 若对原始文件中此三段内容进行扩充(新增非重复信息)，请注意剩余可分配 Byte 数量限制 `65535(2B) - 40397 = 25138`

> n < 64: 为了读取效率，头部信息行长度，已限制不得超过64B，排除行首标志位(1B)，即单行信息应<=63B(目前已有的最长仅47B)，扩充原始文件时需注意。


#### 2. 地域尾部信息
即为前三段后所有内容，目前包括 `城市|isp`，可扩展，由于长度标志位仅1B，扩展后总字符数不可超过255B
长度若在64B(可修改)内（目前原始文件数据均未超过）则同原项目查询一样仅一次io，超过后(<=255B)将增加一次io。

尾部信息行存储结构：
|头部偏移量|信息长度|信息|
|-|-|-|
|2 Bytes|1 Byte|n Bytes(n <= 255)|


> 起始 2B，标志位，用于定位头部信息

> 之后 1B，标志位，用于记录实际字符byte长度(不包含标志位长度，即3)

> n <= 255：尾部信息byte长度可扩展，可以达到1B能代表的最大长度，即 n <= 255，但鉴于目前的尾部信息长度(最长仅42B)，仅尝试读取64B，若扩展后超过此长度，将增加一次io用来获取行缺少的内容


### 缩减二分索引结构行长度，由原来的 14 Bytes 改为 8 Bytes，新结构为
|startIpLongTail|endIpLongTail|regionTailPtr|
|-|-|-|
|2 Bytes|2 Bytes|4 Bytes|
> 由于目前所有缓存模式的查询均使用了vector结构进行第一次寻址，所以二分结构行内去除了重复且无用的ip前两段数值(即舍弃了后续只使用二分索引进行查询的可行性，也不推荐采用这种方式)，保留ip的后两段，即startLongIpTail,endLongIpTail(以ip string为例： ip: 1.2.3.4, ipHead: 1.2; ipTail: 3.4 ipHead将被移除，ipTail转为long存入索引行)

> 地域信息1:n二分索引，所以这里二分索引内的长度段移除，改为记录到地域尾部信息内，节省冗余空间

## 运行数据参照
以下数据，均为本机执行结果，非结论性统计，仅供参考

## License
本项目遵循 MIT License，原项目遵循 Apache License 2.0，详细内容请查看 [LICENSE](./LICENSE)