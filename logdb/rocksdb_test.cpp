//
// compile with:
// 		g++ -std=c++11 -Wall rocksdb_test.cpp \
// 		-I /path/to/rocksdb-rootdirectory/include/ \
// 		-L/path/to/rocksdb-rootdirectory/ \
// 		-lpthread -ldl -lz -lbz2 -llz4 -lzstd -o rocksdb_test
//
#include <iostream>
#include <string>
#include <unistd.h>
#include <time.h>
#include <stdlib.h>
#include <getopt.h>
#include "rocksdb/db.h"
extern char * optarg;

using std::cout;
using std::endl;

int main(int argc, char * argv[])
{
	//default value for args
	int count = 100;
	long unit = 1024 * 1024; // 1MB
	std::string dbpath("rocksdb.db");
	struct timespec starttsp;
	struct timespec endtsp;
	struct timespec elapsed;

	rocksdb::DB * db;
	rocksdb::Options dbopts;
	rocksdb::WriteOptions writeopts;
	rocksdb::Status status;

	std::string * pdata = new std::string(unit, '0');
	
	//is it need to change defualt values for args?
	int opt;
	while((opt = getopt(argc, argv, "c:u:")) != -1)
	{
		switch(opt)
		{
			case 'c':
				count = atoi(optarg);
				break;
			case 'u':
				unit = atol(optarg);
				break;
			default:
				cout << "usage: ./cmd [-c count] [-u unit]\n";
				return 1;
		}
	}

	cout << "loop count: " << count <<
		", chunk size: " << unit << "(in bytes)" << endl;
	
	//create database
	dbopts.create_if_missing = true;
	status = rocksdb::DB::Open(dbopts, dbpath, &db);
	if(!status.ok())
	{
		cout << "Open: " << status.ToString() << endl;
		return 1;
	}
	
	//start test
	writeopts.sync = true;
	clock_gettime(CLOCK_REALTIME, &starttsp);
	for(int i = 0; i < count; i++)
	{
		//auto-sync after each insertion
		status = db->Put(writeopts, std::to_string(i), *pdata);
		if(!status.ok())
		{
			cout << "Put: " << status.ToString() << endl;
			return 1;
		}
	}
	clock_gettime(CLOCK_REALTIME, &endtsp);
	elapsed.tv_sec = endtsp.tv_sec - starttsp.tv_sec;
	elapsed.tv_nsec = endtsp.tv_nsec - starttsp.tv_nsec;
	double totaltime = elapsed.tv_sec + ((double)elapsed.tv_nsec / 1e9);
	
	
	std::printf("total time spend to append+sync all data: %.6lfs\n", totaltime);
	std::printf("average time spend to write %.3lfKB: %.6lfs\n",
				(double)unit / 1024, totaltime / count);
	std::printf("rough throughput: %.3lf MB per second\n",
				((double)unit * count / (1024*1024)) / totaltime);
	
	delete pdata;
	//close db
	delete db;
	return 0;
}
