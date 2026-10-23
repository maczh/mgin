# 工具库

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 加密、摘要、JWT、文件、时间、集合、系统等无状态工具。

## `github.com/maczh/mgin/v2/pkg/utils`

```go
import "github.com/maczh/mgin/v2/pkg/utils"
```

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `LimitQueue` | = &LimitQueeMap{ LimitQueue: make(map[string][]int64), } |  |
| `RSA` | = &RSASecurity{} |  |
| `ErrDataToLarge` | = errors.New("message too long for RSA public key size") |  |
| `ErrDataLen` | = errors.New("data length error") |  |
| `ErrDataBroken` | = errors.New("data broken, first byte is not zero") |  |
| `ErrKeyPairDismatch` | = errors.New("data is not encrypted by the private key") |  |
| `ErrDecryption` | = errors.New("decryption error") |  |
| `ErrPublicKey` | = errors.New("get public key error") |  |
| `ErrPrivateKey` | = errors.New("get private key error") |  |
| `UUIDSnippet` | = block1 + "-" + block2 + "-" + block3 + "-" + block4 + "-" + block5 | regex for validating that the UUID matches RFC 4122. This package generates version 4 UUIDs but accepts any UUID version. http://www.ietf.org/rfc/rfc4122.txt |

### 类型

#### `ExpireCache`

```go
type ExpireCache struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Timeout` | int64 |  |  |

**方法**

```go
func (c *ExpireCache) Delete(key string)
```

```go
func (c *ExpireCache) Load(key string) (any, bool)
```

```go
func (c *ExpireCache) Store(key string, value any)
```

#### `HashSet`

```go
type HashSet struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (set *HashSet) Add(i string) bool
```

```go
func (set *HashSet) Exists(i string) bool
```

```go
func (set *HashSet) Remove(i string)
```

```go
func (set *HashSet) Members() []string
```

#### `LimitQueeMap`

```go
type LimitQueeMap struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `sync.RWMutex` | sync.RWMutex |  |  |
| `LimitQueue` | map[string][]int64 |  |  |

#### `LinkList`

```go
type LinkList[T any] struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (l *LinkList[T]) Add(v T, num int)
```

Add 在指定下标处插入节点。
num < 0 或 num > Size() 时不做插入（不再 panic）。
num == Size() 时追加到末尾。

```go
func (l *LinkList[T]) Remove(index int)
```

Remove 移除指定下标的节点。
下标越界（index < 0 或 index >= Size()）时不做任何操作，不会 panic 也不会误删其他节点。
注意：index == 0 沿用历史行为，等价于 Pop（移除尾节点）。

```go
func (l *LinkList[T]) Push(v T)
```

```go
func (l *LinkList[T]) Pop()
```

```go
func (l *LinkList[T]) Enqueue(v T)
```

```go
func (l *LinkList[T]) Dequeue()
```

```go
func (l *LinkList[T]) Size() int
```

```go
func (l *LinkList[T]) Get(index int) T
```

Get 返回指定下标的节点值。
下标越界（index < 0 或 index >= Size()）时不再 panic，而是返回 T 的零值。
如需区分「越界」与「元素值恰好是零值」，请改用 TryGet。

```go
func (l *LinkList[T]) TryGet(index int) (T, bool)
```

TryGet 尝试返回指定下标的节点值。
第二个返回值表示下标是否有效：false 表示越界，此时第一个返回值为 T 的零值。
该函数不会 panic，调用方可以显式处理越界场景。

```go
func (l *LinkList[T]) GetAll() []T
```

```go
func (l *LinkList[T]) Walk(fn func(v T) bool)
```

#### `Pair`

```go
type Pair struct
```

按值排序

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Key` | string | json:"key" |  |
| `Value` | any | json:"value" |  |

#### `PairList`

```go
type PairList []Pair
```

**方法**

```go
func (p PairList) Len() int
```

```go
func (p PairList) Less(i, j int) bool
```

```go
func (p PairList) Swap(i, j int)
```

#### `RingBuffer`

```go
type RingBuffer[T any] struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Size` | int32 |  |  |
| `Reader` | int32 |  |  |
| `Writer` | int32 |  |  |
| `Buffer` | []T |  |  |

**方法**

```go
func (r *RingBuffer[T]) Write(value T)
```

```go
func (r *RingBuffer[T]) Read() T
```

```go
func (r *RingBuffer[T]) Latest() T
```

```go
func (r *RingBuffer[T]) Oldest() T
```

```go
func (r *RingBuffer[T]) Overwrite(v T)
```

#### `RSASecurity`

```go
type RSASecurity struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (rsas *RSASecurity) SetPublicKey(pubStr string) (err error)
```

设置公钥

```go
func (rsas *RSASecurity) SetPrivateKey(priStr string) (err error)
```

设置私钥

```go
func (rsas *RSASecurity) GetPrivatekey() (*rsa.PrivateKey, error)
```

*rsa.PublicKey

```go
func (rsas *RSASecurity) GetPublickey() (*rsa.PublicKey, error)
```

*rsa.PrivateKey

```go
func (rsas *RSASecurity) PubKeyENCTYPT(input []byte) ([]byte, error)
```

公钥加密

```go
func (rsas *RSASecurity) PubKeyDECRYPT(input []byte) ([]byte, error)
```

公钥解密

```go
func (rsas *RSASecurity) PriKeyENCTYPT(input []byte) ([]byte, error)
```

私钥加密

```go
func (rsas *RSASecurity) PriKeyDECRYPT(input []byte) ([]byte, error)
```

私钥解密

```go
func (rsas *RSASecurity) SignSha1WithRsa(data string) (string, error)
```

*
 * 使用RSAWithSHA1算法签名

```go
func (rsas *RSASecurity) SignSha256WithRsa(data string) (string, error)
```

*
 * 使用RSAWithSHA256算法签名

```go
func (rsas *RSASecurity) SignSha256WithRsaHex(data string) (string, error)
```

```go
func (rsas *RSASecurity) SignSha256WithRsaUrlSafe(data string) (string, error)
```

```go
func (rsas *RSASecurity) VerifySignSha1WithRsa(data string, signData string) error
```

*
 * 使用RSAWithSHA1验证签名

```go
func (rsas *RSASecurity) VerifySignSha256WithRsa(data string, signData string) error
```

*
 * 使用RSAWithSHA256验证签名

#### `Map`

```go
type Map[T any] struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *Map[T]) Load(name string) *T
```

```go
func (c *Map[T]) Store(name string, value *T)
```

```go
func (c *Map[T]) Range(iterator func(name string, item *T) bool)
```

```go
func (c *Map[T]) Delete(name string)
```

```go
func (c *Map[T]) DeleteDirectly(name string)
```

```go
func (c *Map[T]) LoadAndStore(name string, value *T) *T
```

```go
func (c *Map[T]) LoadAndDelete(name string) *T
```

```go
func (c *Map[T]) Len() int
```

```go
func (c *Map[T]) Map() map[string]*T
```

```go
func (c *Map[T]) Clear()
```

#### `WeekDate`

```go
type WeekDate struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `WeekTh` | int |  |  |
| `StartDate` | string |  |  |
| `EndDate` | string |  |  |

#### `EscapeError`

```go
type EscapeError string
```

**方法**

```go
func (e EscapeError) Error() string
```

#### `UUID`

```go
type UUID [16]byte
```

UUID represent a universal identifier with 16 octets.

**方法**

```go
func (uuid UUID) Copy() UUID
```

Copy returns a copy of the UUID.

```go
func (uuid UUID) Raw() [16]byte
```

Raw returns a copy of the UUID bytes.

```go
func (uuid UUID) String() string
```

String returns a hexadecimal string representation with
standardized separators.

```go
func (uuid UUID) Simple() string
```

#### `Values`

```go
type Values struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (p *Values) Put(id string, val any)
```

```go
func (p *Values) Get(id string) any
```

```go
func (p *Values) GetAll() any
```

```go
func (p *Values) Merge(props map[string]any)
```

```go
func (p *Values) Clear()
```

### 函数

#### `AESBase64Decrypt`

```go
func AESBase64Decrypt(encrypt_data string, key string, iv []byte) (string, error)
```

#### `AESBase64Encrypt`

```go
func AESBase64Encrypt(origin_data string, key string, iv []byte) (base64_result string, err error)
```

#### `AESDecrypt`

```go
func AESDecrypt(data string, key string) string
```

#### `AESEncrypt`

```go
func AESEncrypt(data string, key string) string
```

#### `AddPortsToFirewall`

```go
func AddPortsToFirewall(ports []int)
```

#### `AddSpaceBetweenCharsAndNumbers`

```go
func AddSpaceBetweenCharsAndNumbers(src string) string
```

#### `AddString`

```go
func AddString(s ...string) string
```

AddString 拼接字符串

#### `AddStringBytes`

```go
func AddStringBytes(s ...string) []byte
```

AddStringBytes 拼接字符串, 返回 bytes from bytes.Join()

#### `AesDecrypt`

```go
func AesDecrypt(encrypted []byte, key []byte) ([]byte, error)
```

#### `AesDecryptEcb`

```go
func AesDecryptEcb(crypted, key string) (string, error)
```

#### `AesEncrypt`

```go
func AesEncrypt(origData []byte, key []byte) ([]byte, error)
```

#### `AesEncryptEcb`

```go
func AesEncryptEcb(src, key string) (string, error)
```

#### `AnyToMap`

```go
func AnyToMap(obj any) map[string]string
```

#### `AnyToString`

```go
func AnyToString(i any) (string, error)
```

#### `AppDir`

```go
func AppDir() string
```

#### `AppName`

```go
func AppName() string
```

#### `ArrayInt2Str`

```go
func ArrayInt2Str(data []int) []string
```

[]int => []string

#### `ArrayStr2Int`

```go
func ArrayStr2Int(data []string) []int
```

[]string => []int

#### `Base64Decode`

```go
func Base64Decode(str string) string
```

#### `Base64Encode`

```go
func Base64Encode(str string) string
```

#### `Basename`

```go
func Basename(file string) string
```

get filepath base name

#### `BytePos`

```go
func BytePos(s string, bt byte) int
```

BytePos alias of the strings.IndexByte

#### `CenterJustin`

```go
func CenterJustin(text string, size int) string
```

Center justifies the text in the center

#### `CheckSqlValidate`

```go
func CheckSqlValidate(content string) (bool, string)
```

CheckSqlValidate 检查是否含有可能产生注入的非法字符
返回值为true时表示含有非法字符，同时返回的字符串值为匹配到的非法字符

#### `ChineseLength`

```go
func ChineseLength(str string) int
```

#### `ClearUtf8BOM`

```go
func ClearUtf8BOM(bomStr string) string
```

#### `Clone`

```go
func Clone(src any, dst any)
```

#### `CmdExec`

```go
func CmdExec(name string, arg ...string) (string, error)
```

#### `CmdRunWithTimeout`

```go
func CmdRunWithTimeout(cmd *exec.Cmd, timeout time.Duration) (error, bool)
```

#### `CompactJSON`

```go
func CompactJSON(in string) string
```

CompactJSON compact json input with insignificant space characters elided

#### `Compress`

```go
func Compress(data []byte) ([]byte, error)
```

Compress returns compressed bytes

#### `ConvertDateFormat`

```go
func ConvertDateFormat(timeStr string, format string) string
```

#### `CopyStruct`

```go
func CopyStruct(src, dst any)
```

#### `DBCtoSBC`

```go
func DBCtoSBC(s string) string
```

全角转半角

#### `DESDecrypt`

```go
func DESDecrypt(data string, key string) string
```

#### `DESDecryptECB`

```go
func DESDecryptECB(data string, key string) string
```

#### `DESEncrypt`

```go
func DESEncrypt(data string, key string) string
```

#### `DESEncryptECB`

```go
func DESEncryptECB(data string, key string) string
```

#### `DateFormat`

```go
func DateFormat(t time.Time, format string) string
```

Format time.Time struct to string
MM - month - 01
M - month - 1, single bit
DD - day - 02
D - day 2
YYYY - year - 2006
YY - year - 06
HH - 24 hours - 03
H - 24 hours - 3
hh - 12 hours - 03
h - 12 hours - 3
mm - minute - 04
m - minute - 4
ss - second - 05
s - second = 5

#### `DecodeXMLToMap`

```go
func DecodeXMLToMap(r io.Reader) (m map[string]string, err error)
```

DecodeXMLToMap decodes xml reading from io.Reader and returns the first-level sub-node key-value set,
if the first-level sub-node contains child nodes, skip it.

#### `Decompress`

```go
func Decompress(data []byte) ([]byte, error)
```

Decompress returns the decompressed bytes

#### `DeepCopy`

```go
func DeepCopy[D any](src any) D
```

#### `DefaultIfNil`

```go
func DefaultIfNil(value any, defaultValue any) any
```

DefaultIfNil checks if the value is nil, if true returns the default value otherwise the original

#### `DeleteArray`

```go
func DeleteArray(src []any, index int) (result []any)
```

删除数组

#### `DesDecrypt`

```go
func DesDecrypt(crypted, key []byte) ([]byte, error)
```

#### `DesEcbDecrypt`

```go
func DesEcbDecrypt(src, key []byte) ([]byte, error)
```

#### `DesEcbEncrypt`

```go
func DesEcbEncrypt(src, key []byte) ([]byte, error)
```

#### `DesEncrypt`

```go
func DesEncrypt(origData, key []byte) ([]byte, error)
```

#### `DifferenceStringSlice`

```go
func DifferenceStringSlice(slice1, slice2 []string) []string
```

求差集 slice1-并集

#### `Dir`

```go
func Dir(file string) string
```

get filepath dir name

#### `DirsUnder`

```go
func DirsUnder(dirPath string) ([]string, error)
```

list dirs under dirPath

#### `DisplaySize`

```go
func DisplaySize(raw float64) string
```

#### `DownloadFile`

```go
func DownloadFile(fileUrl, localPath string) (string, error)
```

#### `EncodeXMLFromMap`

```go
func EncodeXMLFromMap(w io.Writer, m map[string]string, rootname string) (err error)
```

EncodeXMLFromMap encodes map[string]string to io.Writer with xml format.

NOTE: This function requires the rootname argument and the keys of m (type map[string]string) argument
are legitimate xml name string that does not contain the required escape character!

#### `Existi`

```go
func Existi(src map[string]any, key string) bool
```

#### `Exists`

```go
func Exists(src map[string]string, key string) bool
```

#### `Ext`

```go
func Ext(file string) string
```

#### `FileMD5`

```go
func FileMD5(filename string) (string, error)
```

计算文件的MD5值

#### `FileMTime`

```go
func FileMTime(file string) (int64, error)
```

get file modified time

#### `FileSha256`

```go
func FileSha256(filename string) (string, error)
```

#### `FileSize`

```go
func FileSize(file string) (int64, error)
```

get file size as how many bytes

#### `FilesUnder`

```go
func FilesUnder(dirPath string) ([]string, error)
```

list files under dirPath

#### `FirstNonNil`

```go
func FirstNonNil(values ...any) any
```

FirstNonNil returns the first non nil parameter

#### `Float64ArrayContains`

```go
func Float64ArrayContains(src []float64, dst float64) bool
```

#### `Float64ArrayDelete`

```go
func Float64ArrayDelete(src []float64, index int) []float64
```

#### `FromJSON`

```go
func FromJSON(j string, o any) *any
```

#### `GbkToUtf8`

```go
func GbkToUtf8(gbkStr string) (string, error)
```

#### `GenerateRandString`

```go
func GenerateRandString(source string, l int) string
```

#### `GenerateToken`

```go
func GenerateToken(claims jwt.MapClaims) (string, error)
```

GenerateToken 生成JWT token

#### `Get0Hour`

```go
func Get0Hour(t time.Time) time.Time
```

Get0Hour 当天 0 点

#### `Get0LastMonth`

```go
func Get0LastMonth(t time.Time) time.Time
```

Get0LastMonth 上月第一天 0 点

#### `Get0LastWeek`

```go
func Get0LastWeek(t time.Time) time.Time
```

Get0LastWeek 上周一 0 点

#### `Get0Minute`

```go
func Get0Minute(t time.Time) time.Time
```

Get0Minute 0 分

#### `Get0Month`

```go
func Get0Month(t time.Time) time.Time
```

Get0Month 当月第一天 0 点

#### `Get0NextMonth`

```go
func Get0NextMonth(t time.Time) time.Time
```

Get0NextMonth 下月第一天 0 点

#### `Get0NextWeek`

```go
func Get0NextWeek(t time.Time) time.Time
```

Get0NextWeek 下周一 0 点

#### `Get0Second`

```go
func Get0Second(t time.Time) time.Time
```

Get0Second 0 秒

#### `Get0Tomorrow`

```go
func Get0Tomorrow(t time.Time) time.Time
```

Get0Tomorrow 昨天 0 点

#### `Get0Week`

```go
func Get0Week(t time.Time) time.Time
```

Get0Week 本周一 0 点

#### `Get0Yesterday`

```go
func Get0Yesterday(t time.Time) time.Time
```

Get0Yesterday 昨天 0 点

#### `GetDate`

```go
func GetDate() string
```

#### `GetGoroutineID`

```go
func GetGoroutineID() uint64
```

#### `GetIPv4ByInterface`

```go
func GetIPv4ByInterface(name string) ([]string, error)
```

GetIPv4ByInterface return IPv4 address from a specific interface IPv4 addresses

#### `GetLocalIpAddress`

```go
func GetLocalIpAddress() (ip string)
```

#### `GetMonthDays`

```go
func GetMonthDays(t time.Time) int
```

GetMonthDays 当月天数

#### `GetNowDateTime`

```go
func GetNowDateTime() string
```

#### `GetRandomCaseString`

```go
func GetRandomCaseString(l int) string
```

#### `GetRandomHexString`

```go
func GetRandomHexString(l int) string
```

#### `GetRandomIntString`

```go
func GetRandomIntString(l int) string
```

#### `GetRandomString`

```go
func GetRandomString(l int) string
```

#### `GetStructFields`

```go
func GetStructFields(obj any) []string
```

#### `GetStructJsonTags`

```go
func GetStructJsonTags(obj any) []string
```

#### `GetUUIDString`

```go
func GetUUIDString() string
```

#### `GinHeaders`

```go
func GinHeaders(c *gin.Context) map[string]string
```

#### `GinParamMap`

```go
func GinParamMap(c *gin.Context) map[string]string
```

GinParamMap 获取请求参数，转成Map

#### `GormTimeFormat`

```go
func GormTimeFormat(t string) string
```

#### `GroupByWeekDate`

```go
func GroupByWeekDate(startTime, endTime time.Time) []WeekDate
```

将开始时间和结束时间分割为周为单位

#### `HmacSHA1`

```go
func HmacSHA1(key string, data string) []byte
```

#### `HmacSHA1Base64`

```go
func HmacSHA1Base64(key string, data string) string
```

#### `HmacSHA1Hex`

```go
func HmacSHA1Hex(key string, data string) string
```

#### `HmacSHA256`

```go
func HmacSHA256(key string, data string) []byte
```

#### `HmacSHA256Base64`

```go
func HmacSHA256Base64(key string, data string) string
```

#### `HmacSHA256Hex`

```go
func HmacSHA256Hex(key string, data string) string
```

#### `IfThen`

```go
func IfThen(condition bool, a any) any
```

IfThen evaluates a condition, if true returns the parameters otherwise nil

#### `IfThenElse`

```go
func IfThenElse(condition bool, a any, b any) any
```

IfThenElse evaluates a condition, if true returns the first parameter otherwise the second

#### `InsureDir`

```go
func InsureDir(path string) error
```

#### `Int64Unique`

```go
func Int64Unique(a []int64) []int64
```

Int64Unique 去重

#### `IntArrayContains`

```go
func IntArrayContains(src []int, dst int) bool
```

#### `IntArrayDelete`

```go
func IntArrayDelete(src []int, index int) []int
```

#### `IntersectStringSlice`

```go
func IntersectStringSlice(slice1, slice2 []string) []string
```

求交集

#### `IsAlphaNum`

```go
func IsAlphaNum(c uint8) bool
```

IsAlphaNum reports whether the byte is an ASCII letter, number, or underscore

#### `IsAlphabet`

```go
func IsAlphabet(char uint8) bool
```

IsAlphabet char

#### `IsBlank`

```go
func IsBlank(text string) bool
```

IsBlank returns true if the string is blank (all whitespace)

#### `IsBlankBytes`

```go
func IsBlankBytes(bs []byte) bool
```

IsBlankBytes returns true if the given []byte is all space characters.

#### `IsChinaMobile`

```go
func IsChinaMobile(b []byte) bool
```

检验是否为合法的中国手机号, 不是那么太精细
只要是 13,14,15,18 开头的 11 位数字就认为是中国手机号

#### `IsChinaMobileString`

```go
func IsChinaMobileString(s string) bool
```

同 func IsChinaMobile(b []byte) bool

#### `IsChinese`

```go
func IsChinese(str string) bool
```

#### `IsChineseName`

```go
func IsChineseName(b []byte) bool
```

IsChineseName 检验是否为有效的中文姓名(比如 张三, 李四, 张三·李四)

#### `IsChineseNameEx`

```go
func IsChineseNameEx(b []byte) ([]byte, bool)
```

IsChineseNameEx 检验是否为有效的中文姓名(比如 张三, 李四, 张三·李四),
主要功能和 IsChineseName 相同, 但是如果姓名中包含不规范的间隔符, 会自动修正为正确的间隔符 '\u00B7', 并返回正确的结果.

#### `IsChineseNameString`

```go
func IsChineseNameString(s string) bool
```

同 IsChineseName(b []byte) bool

#### `IsChineseNameStringEx`

```go
func IsChineseNameStringEx(s string) (string, bool)
```

同 IsChineseNameEx(b []byte) ([]byte, bool)

#### `IsDir`

```go
func IsDir(s string) bool
```

IsDir 目录是否存在

#### `IsEmpty`

```go
func IsEmpty(text string) bool
```

IsEmpty returns true if the string is empty

#### `IsEndOf`

```go
func IsEndOf(s, sub string) bool
```

IsEndOf alias of the strings.HasSuffix

#### `IsExist`

```go
func IsExist(path string) bool
```

IsExist checks whether a file or directory exists.
It returns false when the file or directory does not exist.

#### `IsFile`

```go
func IsFile(filePath string) bool
```

IsFile checks whether the path is a file,
it returns false when it's a directory or does not exist.

#### `IsIdCard`

```go
func IsIdCard(cardNo string) bool
```

判断是否是18或15位身份证

#### `IsIntranetIP`

```go
func IsIntranetIP(ip string) bool
```

#### `IsMail`

```go
func IsMail(b []byte) bool
```

检验是否为合法的电子邮箱, 考虑到各个网站的 mail 要求不一样, 这里匹配比较宽松
邮箱用户名可以包含 0-9, A-Z, a-z, -, _, .
开头字母不能是 -, _, .
结尾字母不能是 -, _, .
-, _, . 这三个连接字母任意两个不能连续, 如不能出现 --, __, .., -_, -., _.
邮箱的域名可以包含 0-9, A-Z, a-z, -
连接字符 - 只能出现在中间, 不能连续, 如不能 --
支持多级域名, x@y.z, x@y.z.w, x@x.y.z.w.e

#### `IsMailString`

```go
func IsMailString(s string) bool
```

同 func IsMail(b []byte) bool

#### `IsMark`

```go
func IsMark(r rune) bool
```

IsMark determines whether the rune is a marker

#### `IsNickname`

```go
func IsNickname(b []byte) bool
```

检验是否为合法的昵称, 合法的字符有 0-9, A-Z, a-z, _, 汉字
字符 '_' 只能出现在中间且不能重复, 如 "__"

#### `IsNicknameString`

```go
func IsNicknameString(s string) bool
```

同 func IsNickname(b []byte) bool

#### `IsNotBlank`

```go
func IsNotBlank(text string) bool
```

IsNotBlank returns true if the string is not blank

#### `IsNotEmpty`

```go
func IsNotEmpty(text string) bool
```

IsNotEmpty returns true if the string is not empty

#### `IsNumeric`

```go
func IsNumeric(c byte) bool
```

IsNumeric returns true if the given character is a numeric, otherwise false.

#### `IsPortAvailable`

```go
func IsPortAvailable(port int) bool
```

IsPortAvailable 判断本机指定 TCP 端口当前是否可用（即没有被任何进程占用）。

返回 true 表示端口空闲可用，false 表示端口已被占用或端口号非法。
实现方式：尝试监听该端口，成功则说明空闲（随后立即释放），失败则说明被占用。
相比 IsPortUse，该函数不依赖 netstat/findstr 等外部命令，跨平台可用，也不会
因为端口数字出现在 PID 或 IP 地址中而产生误报。

#### `IsPortUse`

```go
func IsPortUse(port int) bool
```

IsPortUse 判断端口是否被占用。

注意：该函数的返回值与其文档注释相反。true 实际表示 netstat 未匹配到该端口（端口空闲），
false 表示 netstat 匹配到了该端口（端口可能已被占用）。

Deprecated: 命名与语义相反，且实现依赖 netstat/grep 外部命令，存在以下已知问题：
1. 端口数字只要出现在 netstat 输出的任意位置（PID、IP 地址、其他端口号如 8080 之于 80）
就会被判定为命中，存在误报；
2. 仅支持 linux 与 windows，在其他系统（如 darwin）上恒返回 true（即"端口空闲"）。

新代码请改用 IsPortAvailable 判断端口是否可用。
本函数保持原有返回值语义与实现不变，以避免破坏现有调用方。

#### `IsSpace`

```go
func IsSpace(c byte) bool
```

IsSpace returns true if the given character is a space, otherwise false.

#### `IsSpaceRune`

```go
func IsSpaceRune(r rune) bool
```

IsSpaceRune returns true if the given rune is a space, otherwise false.

#### `IsStartOf`

```go
func IsStartOf(s, sub string) bool
```

IsStartOf alias of the strings.HasPrefix

#### `IsUserName`

```go
func IsUserName(b []byte) bool
```

检验是否为合法的用户名, 合法的字符有 0-9, A-Z, a-z, _
第一个字母不能为 _, 0-9
最后一个字母不能为 _, 且 _ 不能连续

#### `IsUserNameString`

```go
func IsUserNameString(s string) bool
```

同 func IsName(b []byte) bool

#### `IsValidUUIDString`

```go
func IsValidUUIDString(s string) bool
```

IsValidUUIDString returns true, if the given string matches a valid UUID (version 4, variant 2).

#### `JSONPretty`

```go
func JSONPretty(in, prefix, indent string) string
```

JSONPrettyPrint pretty print raw json string to indent string

#### `Left`

```go
func Left(src string, size int) string
```

#### `LeftJustin`

```go
func LeftJustin(text string, size int) string
```

Left justifies the text to the left

#### `Length`

```go
func Length(text string) int
```

Length counts the input while respecting UTF8 encoding and combined characters

#### `LimitFreqSingle`

```go
func LimitFreqSingle(queueName string, count uint, timeWindow int64) bool
```

单机时间滑动窗口限流法

#### `LoadYaml`

```go
func LoadYaml(filename string, cfg any) error
```

#### `LocalIPv4s`

```go
func LocalIPv4s() ([]string, error)
```

LocalIPs return all non-loopback IPv4 addresses

#### `LowerFirst`

```go
func LowerFirst(s string) string
```

LowerFirst lower first char

#### `Lowercase`

```go
func Lowercase(s string) string
```

#### `MD5Encode`

```go
func MD5Encode(content string) (md string)
```

#### `Map2Struct`

```go
func Map2Struct(input interface{}, output interface{}) error
```

Map2Struct Decode takes an input structure and uses reflection to translate it to
Map 2Struct Decode获取一个输入结构，并使用反射将其转换为
the output structure. output must be a pointer to a map or struct.
输出结构输出必须是指向map或struct的指针。

#### `MapGet`

```go
func MapGet(input interface{}, fieldName string) interface{}
```

Get 获取map中的字段，支持嵌套结构获取，例如fieldName.subFieldName.xx
嵌套类型必须是map[string]interface{}
如果字段不存在，返回nil

#### `MapItoS`

```go
func MapItoS(src map[string]any) map[string]string
```

map转换

#### `MapMD5`

```go
func MapMD5(m map[string]string) string
```

#### `MapStoI`

```go
func MapStoI(src map[string]string) map[string]any
```

map转换

#### `MustNewUUID`

```go
func MustNewUUID() UUID
```

MustNewUUID returns a new uuid, if an error occurs it panics.

#### `NewECBDecrypter`

```go
func NewECBDecrypter(b cipher.Block) cipher.BlockMode
```

NewECBDecrypter returns a BlockMode which decrypts in electronic code book
mode, using the given Block.

#### `NewECBEncrypter`

```go
func NewECBEncrypter(b cipher.Block) cipher.BlockMode
```

NewECBEncrypter returns a BlockMode which encrypts in electronic code book
mode, using the given Block.

#### `NewHashSet`

```go
func NewHashSet() *HashSet
```

#### `NewLimitQueue`

```go
func NewLimitQueue()
```

#### `NewRingBuffer`

```go
func NewRingBuffer[T any](size int) *RingBuffer[T]
```

#### `NewSafeGo`

```go
func NewSafeGo(argsF func(args ...any)) *safeGo
```

NewSafeGo 创建一个安全的协程调用
示例:
	safeGo := NewSafeGo(func(args ...any) {

	})
	safeGo.SetGoBeforeHandler(func() map[string]any {
		return map[string]any{
			"preRoutineId": "123",
		}
	})
	safeGo.SetCallBeforeHandler(func(params map[string]any) {
		fmt.Println(params["preRoutineId"])
	})
	safeGo.Run("hello", "world")

#### `NewUUID`

```go
func NewUUID() (UUID, error)
```

NewUUID generates a new version 4 UUID relying only on random numbers.

#### `NewUUIDString`

```go
func NewUUIDString() string
```

#### `PKCS5Padding`

```go
func PKCS5Padding(cipherText []byte, blockSize int) []byte
```

#### `PKCS5UnPadding`

```go
func PKCS5UnPadding(origData []byte) ([]byte, error)
```

#### `PriKeyDecrypt`

```go
func PriKeyDecrypt(data, privateKey string) (string, error)
```

私钥解密

#### `PriKeyEncrypt`

```go
func PriKeyEncrypt(data, privateKey string) (string, error)
```

私钥加密

#### `PublicDecrypt`

```go
func PublicDecrypt(data, publicKey string) (string, error)
```

公钥解密

#### `PublicEncrypt`

```go
func PublicEncrypt(data, publicKey string) (string, error)
```

公钥加密

#### `ReadFileToBytes`

```go
func ReadFileToBytes(filePath string) ([]byte, error)
```

ReadFileToBytes reads data type '[]byte' from file by given path.
It returns error when fail to finish operation.

#### `ReadFileToString`

```go
func ReadFileToString(filePath string) (string, error)
```

ReadFileToString reads data type 'string' from file by given path.
It returns error when fail to finish operation.

#### `RealPath`

```go
func RealPath(file string) (string, error)
```

get absolute filepath, based on built executable file

#### `Rename`

```go
func Rename(file string, to string) error
```

rename file name

#### `ReplacePunctuation`

```go
func ReplacePunctuation(src, replaceWith string) string
```

#### `ReplacePunctuationWithSpace`

```go
func ReplacePunctuationWithSpace(src string) string
```

#### `Right`

```go
func Right(src string, size int) string
```

#### `RightJustin`

```go
func RightJustin(text string, size int) string
```

Right justifies the text to the right

#### `RunePos`

```go
func RunePos(s string, ru rune) int
```

RunePos alias of the strings.IndexRune

#### `SearchFile`

```go
func SearchFile(filename string, paths ...string) (fullPath string, err error)
```

Search a file in paths.
this is often used in search config file in /etc ~/

#### `SelfDir`

```go
func SelfDir() string
```

SelfDir gets compiled executable file directory

#### `SelfPath`

```go
func SelfPath() string
```

SelfPath gets compiled executable file absolute path

#### `SftpClose`

```go
func SftpClose(sftpClient *sftp.Client, sshClient *ssh.Client)
```

#### `SftpConnect`

```go
func SftpConnect(user, password, host string, port int) (*sftp.Client, *ssh.Client, error)
```

SftpConnect 使用用户名密码建立 sftp 连接。

安全提示（重要）：该函数当前的 HostKeyCallback 恒返回 nil，等价于
ssh.InsecureIgnoreHostKey()，即**不校验服务端主机密钥**，存在中间人攻击（MITM）风险。
这里保留该行为是为了兼容使用自签/未登记主机密钥的存量业务，避免破坏性变更。

新代码请使用 SftpConnectWithHostKey 或 SftpConnectWithKnownHosts 显式校验主机密钥。

#### `SftpConnectWithHostKey`

```go
func SftpConnectWithHostKey(user, password, host string, port int, hostKeyCallback ssh.HostKeyCallback) (*sftp.Client, *ssh.Client, error)
```

SftpConnectWithHostKey 使用用户名密码建立 sftp 连接，并由调用方提供主机密钥校验回调。

hostKeyCallback 为 nil 时回退为 SftpConnect 的不校验行为。
常见的回调实现：
- knownhosts.New("<known_hosts 文件路径>") 基于 known_hosts 文件校验
- ssh.FixedHostKey(pubkey) 固定单一主机密钥

#### `SftpConnectWithKnownHosts`

```go
func SftpConnectWithKnownHosts(user, password, host string, port int, knownHostsPath string) (*sftp.Client, *ssh.Client, error)
```

SftpConnectWithKnownHosts 使用用户名密码建立 sftp 连接，并基于 known_hosts 文件校验主机密钥。

knownHostsPath 为空时，默认使用当前用户家目录下的 ~/.ssh/known_hosts。
主机密钥不在 known_hosts 中或不匹配时，连接会被拒绝并返回错误。

#### `SftpUploadFile`

```go
func SftpUploadFile(sftpClient *sftp.Client, localFilePath string, remotePath string)
```

#### `Sha1`

```go
func Sha1(src string) string
```

#### `Sha256`

```go
func Sha256(src string) string
```

#### `SimpleUUID`

```go
func SimpleUUID() string
```

#### `SliceContains`

```go
func SliceContains(sl []any, v any) bool
```

#### `SliceContainsInt`

```go
func SliceContainsInt(sl []int, v int) bool
```

#### `SliceContainsInt64`

```go
func SliceContainsInt64(sl []int64, v int64) bool
```

#### `SliceContainsString`

```go
func SliceContainsString(sl []string, v string) bool
```

#### `SliceMerge`

```go
func SliceMerge(slice1, slice2 []any) (c []any)
```

SliceMerge merges interface slices to one slice.

#### `SliceMergeInt`

```go
func SliceMergeInt(slice1, slice2 []int) (c []int)
```

#### `SliceMergeInt64`

```go
func SliceMergeInt64(slice1, slice2 []int64) (c []int64)
```

#### `SliceMergeString`

```go
func SliceMergeString(slice1, slice2 []string) (c []string)
```

#### `SliceSumFloat64`

```go
func SliceSumFloat64(intslice []float64) (sum float64)
```

#### `SliceSumInt`

```go
func SliceSumInt(intslice []int) (sum int)
```

#### `SliceSumInt64`

```go
func SliceSumInt64(intslice []int64) (sum int64)
```

#### `SliceUniqueInt`

```go
func SliceUniqueInt(s []int) []int
```

#### `SliceUniqueInt64`

```go
func SliceUniqueInt64(s []int64) []int64
```

#### `SliceUniqueString`

```go
func SliceUniqueString(s []string) []string
```

#### `SortMapByValue`

```go
func SortMapByValue(src map[string]any) PairList
```

#### `SortMapByValueDesc`

```go
func SortMapByValueDesc(src map[string]any) PairList
```

#### `StoreYaml`

```go
func StoreYaml(filename string, cfg any) error
```

#### `StrPos`

```go
func StrPos(s, sub string) int
```

StrPos alias of the strings.Index

#### `StringArrayContains`

```go
func StringArrayContains(src []string, dst string) bool
```

#### `StringArrayDelete`

```go
func StringArrayDelete(src []string, index int) []string
```

#### `StringToAny`

```go
func StringToAny[T any](src string) (T, error)
```

#### `StringUnique`

```go
func StringUnique(a []string) []string
```

StringUnique 去重

#### `Struct2Map`

```go
func Struct2Map(obj any) map[string]any
```

Struct2Map return map

#### `Struct2MapString`

```go
func Struct2MapString(obj any) map[string]string
```

#### `Struct2StringMap`

```go
func Struct2StringMap(input any) (map[string]string, error)
```

#### `SubChineseString`

```go
func SubChineseString(str string, begin, length int) string
```

#### `TimeSubDays`

```go
func TimeSubDays(t1, t2 time.Time) int
```

#### `ToDateString`

```go
func ToDateString(date time.Time) string
```

#### `ToDateTimeString`

```go
func ToDateTimeString(date time.Time) string
```

#### `ToJSON`

```go
func ToJSON(o any) string
```

#### `ToJSONPretty`

```go
func ToJSONPretty(o any) string
```

#### `ToTimeString`

```go
func ToTimeString(date time.Time) string
```

#### `TrimEmoji`

```go
func TrimEmoji(s string) string
```

清除emoji表情

#### `TrimSpaceStrInArray`

```go
func TrimSpaceStrInArray(str string, data []string) bool
```

str[TrimSpace] in string list

#### `TripleDESDecrypt`

```go
func TripleDESDecrypt(data string, key string, pcks5padding bool) string
```

#### `TripleDESEncrypt`

```go
func TripleDESEncrypt(data string, key string, pcks5padding bool) string
```

#### `TripleDesDecrypt`

```go
func TripleDesDecrypt(crypted, key []byte, pcks5padding bool) ([]byte, error)
```

3DES解密

#### `TripleDesEncrypt`

```go
func TripleDesEncrypt(origData, key []byte, pcks5padding bool) ([]byte, error)
```

3DES加密

#### `TripleEcbDesDecrypt`

```go
func TripleEcbDesDecrypt(crypted, key []byte) ([]byte, error)
```

[golang ECB 3DES Decrypt]

#### `TripleEcbDesEncrypt`

```go
func TripleEcbDesEncrypt(origData, key []byte) ([]byte, error)
```

[golang ECB 3DES Encrypt]

#### `UUIDFromString`

```go
func UUIDFromString(s string) (UUID, error)
```

#### `UnSplitString`

```go
func UnSplitString(src []string, sep string) string
```

#### `UnicodeEmojiCode`

```go
func UnicodeEmojiCode(s string) string
```

表情转换

#### `UnicodeEmojiDecode`

```go
func UnicodeEmojiDecode(s string) string
```

表情解码

#### `UnionStringSlice`

```go
func UnionStringSlice(slice1, slice2 []string) []string
```

#### `Unlink`

```go
func Unlink(file string) error
```

delete file

#### `UpperFirst`

```go
func UpperFirst(s string) string
```

UpperFirst upper first char

#### `UpperWord`

```go
func UpperWord(s string) string
```

UpperWord Change the first character of each word to uppercase

#### `Uppercase`

```go
func Uppercase(s string) string
```

Uppercase alias of the strings.ToUpper()

#### `UrlDecode`

```go
func UrlDecode(encoded string) (string, error)
```

#### `UrlEncode`

```go
func UrlEncode(rawString string) string
```

#### `Utf8ToGbk`

```go
func Utf8ToGbk(uft8Str string) string
```

#### `Utf8len`

```go
func Utf8len(s string) int
```

Utf8Len of the string

#### `ValidUtf8String`

```go
func ValidUtf8String(s string) bool
```

ValidUtf8String check

#### `ValidateToken`

```go
func ValidateToken(tokenStr string) (*jwt.Token, error)
```

ValidateToken 验证JWT token

#### `WaitNextMinute`

```go
func WaitNextMinute()
```

WaitNextMinute 下一分钟, 对齐时间, 0 秒

#### `WeekByDate`

```go
func WeekByDate(t time.Time) int
```

判断时间是当年的第几周

#### `WriteBytesToFile`

```go
func WriteBytesToFile(filePath string, b []byte) (int, error)
```

WriteBytesToFile saves content type '[]byte' to file by given path.
It returns error when fail to finish operation.

#### `WriteStringToFile`

```go
func WriteStringToFile(filePath string, s string) (int, error)
```

WriteStringFile saves content type 'string' to file by given path.
It returns error when fail to finish operation.

#### `ZeroPadding`

```go
func ZeroPadding(ciphertext []byte, blockSize int) []byte
```

#### `ZeroUnPadding`

```go
func ZeroUnPadding(origData []byte) []byte
```

#### `ZipFiles`

```go
func ZipFiles(filename string, files []string, srcpath string, aliasnames []string) error
```

ZIP压缩多个文件，带批量修改压缩包里的文件名功能

