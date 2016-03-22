#!/sh

set -e -x

# convert the list of servers on $NTP_SERVERS to use multiple -p command
# arguments.
SERVERS=""
for i in $NTP_SERVERS; do
    SERVERS="-p $i $SERVERS"
done

exec /ntpd -n -N $SERVERS
