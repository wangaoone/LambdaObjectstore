from ..client import NewClient
from ..go import Slice_string, Slice_byte


key      = "foo"
d        = 2
p        = 1
getonly  = False
addrList = "127.0.0.1:6378"

# initial object with random value
val = bytes("Hello infinity!", 'utf-8')

# parse server address
addrArr = Slice_string(addrList.split(","))

# initial new ecRedis client
cli = NewClient(d, p, 32)

# start dial and PUT/GET
cli.Dial(addrArr)
if not getonly:
  cli.Set(key, Slice_byte(val))

err = None
buf = cli.GetVal(key)
if err != None:
  print("Internal error:{}".format(err))
else:
  ret = bytes(buf).decode("utf-8")
  print("GET {}:{}\n".format(key, ret))
