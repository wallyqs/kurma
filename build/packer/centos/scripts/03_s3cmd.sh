set -e -x

yum -y install python-setuptools

cd /tmp
wget https://github.com/s3tools/s3cmd/releases/download/v1.6.1/s3cmd-1.6.1.tar.gz
tar xzvf s3cmd-1.6.1.tar.gz
cd s3cmd-1.6.1
python setup.py install
