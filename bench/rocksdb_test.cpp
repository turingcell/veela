//
// compile with:
// 		g++ -std=c++11 -Wall rocksdb_test.cpp \
// 		-I /path/to/rocksdb-rootdirectory/include/ \
// 		-L/path/to/rocksdb-rootdirectory/ \
// 		-lrocksdb -lpthread -ldl -lz -lbz2 -llz4 -lzstd \
// 		-o rocksdb_test
//
#include <iostream>
#include <string>

#include <sys/types.h>
#include <sys/stat.h>
#include <unistd.h>
#include <fts.h>
#include <time.h>
#include <stdlib.h>
#include <getopt.h>
#include "rocksdb/db.h"
extern char * optarg;
extern int rmdir_r(const char *dir);

using std::cout;
using std::endl;

int main(int argc, char * argv[])
{
	//default value for args
	int count = 100;
	long unit = 1024 * 1024; // 1MB
	std::string dbpath("/tmp/rocksdb.db");
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
	
	if(rmdir_r(dbpath.c_str()) != 0)
		cout << "temporary database files '"
			<< dbpath << "' cannot remove\n";
	delete pdata;
	//close db
	delete db;
	return 0;
}

//delete directory recursively.
//helper function browsed from internel library
int rmdir_r(const char *dir)
{
	int ret = 0;
	FTS *ftsp = NULL;
	FTSENT *curr;

	char *files[] = {(char *)dir, NULL};

	// FTS_NOCHDIR  - Avoid changing cwd, which could cause unexpected behavior
	//                in multithreaded programs
	// FTS_PHYSICAL - Don't follow symlinks. Prevents deletion of files outside
	//                of the specified directory
	// FTS_XDEV     - Don't cross filesystem boundaries
	ftsp = fts_open(files, FTS_NOCHDIR | FTS_PHYSICAL | FTS_XDEV, NULL);
	if (!ftsp) {
		fprintf(stderr, "%s: fts_open failed: %s\n", dir, strerror(errno));
		ret = -1;
		goto finish;
	}

	while ((curr = fts_read(ftsp))) {
		switch (curr->fts_info) {
			case FTS_NS:
			case FTS_DNR:
			case FTS_ERR:
				fprintf(stderr, "%s: fts_read error: %s\n",
					curr->fts_accpath, strerror(curr->fts_errno));
				break;

			case FTS_DC:
			case FTS_DOT:
			case FTS_NSOK:
				// Not reached unless FTS_LOGICAL, FTS_SEEDOT, or FTS_NOSTAT were
				// passed to fts_open()
				break;

			case FTS_D:
				// Do nothing. Need depth-first search, so directories are deleted
				// in FTS_DP
				break;

			case FTS_DP:
			case FTS_F:
			case FTS_SL:
			case FTS_SLNONE:
			case FTS_DEFAULT:
				if (remove(curr->fts_accpath) < 0) {
					fprintf(stderr, "%s: Failed to remove: %s\n",
						curr->fts_path, strerror(curr->fts_errno));
					ret = -1;
				}
				break;
		}
	}

finish:
	if (ftsp) {
		fts_close(ftsp);
	}

	return ret;
}
