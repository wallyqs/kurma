# Abort on error
set -e -x

# We use checksums even when the value is just one we derive, which doesn't
# prove untampered from maintainer, but proves no corruption in download and
# that the code is the _same_ as the version we saw.
apv_yaml=0.1.6
aps_yaml='7da6971b4bd08a986dd2a61353bc422362bd0edcc67d7ebaac68c95f74182749' # per manual download
apv_ruby=2.2.4
apsubdir_ruby=2.2
aps_ruby='b6eff568b48e0fda76e5a36333175df049b204e91217aa32a65153cc0cdcb761'
 # ruby checksum from https://www.ruby-lang.org/en/downloads/
 # be sure to get the checksum for the right compression format
apv_rubygems=2.6.1
aps_rubygems='c9c4d1a8367a1c05bc568fa0eb5c830974d0f328dd73827cc129c0905aae1f4f'  # per manual download

apt-get -y install build-essential libreadline-gplv2-dev zlib1g-dev libssl-dev

download() {
	local base="$1" fn="$2" cksum_want="$3"
	wget "$base/$fn"
	have=$(openssl sha -r -sha256 < "$fn" | cut -d ' ' -f 1)
	if [ ".$have" != ".$cksum_want" ]; then
		echo >&2 "CHECKSUM MISMATCH FOR FILE $fn"
		echo >&2 "    Expected: $cksum_want"
		echo >&2 "         Got: $have"
		exit 1
	fi
}

cd /tmp

# Install Ruby from source in /opt so that users of Vagrant
# can install their own Rubies using packages or however.
download http://pyyaml.org/download/libyaml yaml-${apv_yaml}.tar.gz $aps_yaml
tar xzf yaml-${apv_yaml}.tar.gz
cd yaml-${apv_yaml}
YAMLDIR=`pwd`
./configure --disable-shared --with-pic
make
cd ..

# build ruby
download http://cache.ruby-lang.org/pub/ruby/$apsubdir_ruby ruby-${apv_ruby}.tar.gz $aps_ruby
tar xzf ruby-${apv_ruby}.tar.gz
cd ruby-${apv_ruby}
OLDLD=$LDFLAGS
OLDCPP=$CPPFLAGS
export LDFLAGS="-L$YAMLDIR/src/.libs $LDFLAGS"
export CPPFLAGS="-I$YAMLDIR/include $CPPFLAGS"
./configure --prefix=/opt/ruby --enable-shared --disable-install-doc
make
make install
export LDFLAGS=$OLDLD
export CPPFLAGS=$OLDCPP
cd ..
rm -rf yaml-${apv_yaml} yaml-${apv_yaml}.tar.gz ruby-${apv_ruby} ruby-${apv_ruby}.tar.gz

# Install rubygems
download http://production.cf.rubygems.org/rubygems rubygems-${apv_rubygems}.tgz $aps_rubygems
tar xzf rubygems-${apv_rubygems}.tgz
cd rubygems-${apv_rubygems}
/opt/ruby/bin/ruby setup.rb
cd ..
rm -rf rubygems-${apv_rubygems} rubygems-${apv_rubygems}.tgz

# install bundler
/opt/ruby/bin/gem install bundler -v 1.11.2 --no-ri --no-rdoc

# Symlinks
ln -s /opt/ruby/bin/ruby /usr/local/bin/ruby
ln -s /opt/ruby/bin/gem /usr/local/bin/gem
ln -s /opt/ruby/bin/bundle /usr/local/bin/bundle

# cleanup
apt-get -y remove build-essential libreadline-gplv2-dev
