# execute this script at the root of the veela source directory
cd proto

echo ''' 
veela
''' |grep -Po "[a-zA-Z0-9_]+"| while read package
do
    echo cd $package
    cd $package
    pwd
    protoc --gogofaster_out=. *.proto
    cd -
done
