Content-Type: multipart/mixed; boundary="//"
MIME-Version: 1.0

--//
Content-Type: text/cloud-config; charset="us-ascii"
MIME-Version: 1.0
Content-Transfer-Encoding: 7bit
Content-Disposition: attachment; filename="cloud-config.txt"

#cloud-config
cloud_final_modules:
- [scripts-user, always]

--//
Content-Type: text/x-shellscript; charset="us-ascii"
MIME-Version: 1.0
Content-Transfer-Encoding: 7bit
Content-Disposition: attachment; filename="userdata.txt"

#!/bin/bash -x
wget -O /home/ubuntu/rapidchain https://rapidchain-bucket.s3.amazonaws.com/rapidchain > /tmp/wget.log
chmod +x /home/ubuntu/rapidchain > /tmp/chmod.log
ulimit -n 35000
/home/ubuntu/rapidchain > /tmp/rapidchain.log

--//