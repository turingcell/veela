package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/cockroachdb/pebble"
	"github.com/prologic/bitcask"
)

func ParseUnit(punit *string) (int64, error) {
	var err error
	var mached bool
	var unitTmp int
	var unit int64

	//parse chunk size
	mached, err = regexp.MatchString(`^[1-9]([0-9])*(GB|gb|MB|mb|KB|kb|B|b)`, *punit)
	if mached != true {
		return 0, errors.New("syntax error\n")
	}
	var suffix = (*punit)[strings.IndexAny(*punit, "GMKBgmkb"):]

	unitTmp, err = strconv.Atoi(strings.Trim(*punit, "GMKBgmkb"))
	if err != nil {
		return 0, errors.New("syntax error\n")
	}
	unit = int64(unitTmp)

	switch suffix {
	case "GB", "gb":
		return unit * 1024 * 1024 * 1024, nil
	case "MB", "mb":
		return unit * 1024 * 1024, nil
	case "KB", "kb":
		return unit * 1024, nil
	default:
		return unit, nil
	}
}

//test function for boltdb
//count: loop times
//data: data to write as value in each loop
//return: total time spend if no error occured
func TestBolt(count int, data []byte) (time.Duration, error) {
	var err error
	var db *bolt.DB
	var interval time.Duration
	var filename = "/tmp/bolt.db"

	//create the new data file in current directory.
	os.Remove(filename)
	db, err = bolt.Open(filename, 0600, nil)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	//prepare test bucket.
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte("testbucket"))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return time.Duration(0), err
	}

	//loop test
	interval = time.Duration(0)
	for i := 0; i < count; i++ {
		ret, err := AppendAndSyncForBolt(db, data)
		if err != nil {
			return time.Duration(0), err
		}
		interval += ret
	}
	db.Close()
	os.Remove(filename)

	return interval, nil
}

func AppendAndSyncForBolt(db *bolt.DB, data []byte) (time.Duration, error) {
	var err error
	var start time.Time
	var end time.Time

	start = time.Now()
	//wrap all write operations in a read-write transaction
	err = db.Update(func(tx *bolt.Tx) error {
		//starting index(key) to be written into boltdb
		//all kv-pairs to be written will be in the group: {{1, data}, {2, data}, ...,{*pcount - 1, data}} and will be written in order of incrementing key
		b := tx.Bucket([]byte("testbucket"))
		key, err := b.NextSequence() // begin from 1
		if err != nil {
			return err
		}

		//start!
		err = b.Put([]byte(strconv.FormatUint(key, 10)), data)
		if err != nil {
			return err
		}
		return nil
	})
	end = time.Now()
	if err != nil {
		return time.Duration(0), err
	}
	return end.Sub(start), nil
}

//test direct file append+sync
//count: loop count
//data: data to write on each loop
//return: total time spend if no error occured
func TestRawOP(count int, data []byte) (time.Duration, error) {
	var err error
	var interval time.Duration
	var fp *os.File
	var filename = "/tmp/rawop.db"

	//prepare test file
	os.Remove(filename)
	fp, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return time.Duration(0), err
	}

	//start!
	interval = time.Duration(0)
	for i := 0; i < count; i++ {
		ret, err := AppendAndSyncForRawOP(fp, int64(i), data)
		if err != nil {
			return time.Duration(0), err
		}
		interval += ret
	}

	err = fp.Close()
	if err != nil {
		return time.Duration(0), err
	}
	os.Remove(filename)
	return interval, nil
}
func AppendAndSyncForRawOP(fp *os.File, key int64, data []byte) (time.Duration, error) {
	var start time.Time
	var err error

	start = time.Now()
	_, err = fp.Write([]byte(strconv.FormatInt(key, 10)))
	if err != nil {
		return time.Duration(0), err
	}
	_, err = fp.Write(data)
	if err != nil {
		return time.Duration(0), err
	}
	err = fp.Sync()
	if err != nil {
		return time.Duration(0), err
	}
	return time.Now().Sub(start), nil
}

//test preallocated file append+fdatasync
//count: loop count
//data: data to write on each loop
//return: total time spend if no error occured
func TestRawFdatasyncOP(count int, data []byte) (time.Duration, error) {
	var err error
	var interval time.Duration
	var fp *os.File

	//prepare test file
	os.Remove("rawop.db")
	sz := count * (len(data) + len(string(count)))
	fp, err = os.OpenFile("rawop.db", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return time.Duration(0), err
	}
	ret, err := fp.Seek(int64(sz), os.SEEK_SET)
	if ret != int64(sz) || err != nil {
		fmt.Println("fp.Seek:", ret, err)
		panic("err")
	}
	writed, err := fp.Write([]byte("1"))
	if writed != 1 || err != nil {
		fmt.Println("fp.Write:", writed, err)
		panic("err")
	}
	t1 := time.Now()
	err = fp.Sync()
	t2 := time.Now()
	if err != nil {
		fmt.Println("fp.Sync:", err)
		panic("err")
	} else {
		fmt.Println("takes", t2.Sub(t1), "sync", sz, "bytes lseek file")
	}
	ret, err = fp.Seek(0, os.SEEK_SET)
	if ret != 0 || err != nil {
		fmt.Println("fp.Seek2:", ret, err)
		panic("err")
	}

	//start!
	interval = time.Duration(0)
	for i := 0; i < count; i++ {
		ret, err := AppendAndSyncForRawFdatasyncOP(fp, int64(i), data)
		if err != nil {
			return time.Duration(0), err
		}
		interval += ret
	}

	err = fp.Close()
	if err != nil {
		return time.Duration(0), err
	}
	return interval, nil
}
func AppendAndSyncForRawFdatasyncOP(fp *os.File, key int64, data []byte) (time.Duration, error) {
	var start time.Time
	var err error

	start = time.Now()
	_, err = fp.Write([]byte(strconv.FormatInt(key, 10)))
	if err != nil {
		return time.Duration(0), err
	}
	_, err = fp.Write(data)
	if err != nil {
		return time.Duration(0), err
	}
	//err = fp.Sync()
	fd := fp.Fd()
	err = syscall.Fdatasync(int(fd))
	if err != nil {
		return time.Duration(0), err
	}
	return time.Now().Sub(start), nil
}

//test func for bitcask
//count: loop count
//data: data to write in each loop
//conf: configuration specified for bitcask
//return: total time if no error occured
type BitcaskConfig struct {
	fileSize int
	keySize  uint32
	valSize  uint64
}

func TestBitcask(count int, data []byte, conf *BitcaskConfig) (time.Duration, error) {
	var db *bitcask.Bitcask
	var err error
	var elapsed time.Duration
	var start time.Time
	var filename = "/tmp/bitcask.db"

	//open file with specified config.
	//auto-syncronization after each insert is enabled
	os.RemoveAll(filename)
	db, err = bitcask.Open(filename,
		bitcask.WithSync(true),
		bitcask.WithMaxDatafileSize(conf.fileSize),
		bitcask.WithMaxKeySize(conf.keySize),
		bitcask.WithMaxValueSize(conf.valSize))
	if err != nil {
		return time.Duration(0), err
	}
	//remove all keys before insert
	err = db.DeleteAll()
	if err != nil {
		return time.Duration(0), err
	}

	//start
	start = time.Now()
	for i := 0; i < count; i++ {
		//fmt.Printf("write %d'th record\n", i)
		err := db.Put([]byte(strconv.Itoa(i)), data)
		if err != nil {
			return time.Duration(0), err
		}
	}
	elapsed = time.Now().Sub(start)

	if err = db.Close(); err != nil {
		return time.Duration(0), err
	}
	os.RemoveAll(filename)
	return elapsed, nil
}

//test function for pebble
//count: loop count
//data: data to write on each loop
//return: total time spend to write all data if no error occured
func TestPebble(count int, data []byte) (time.Duration, error) {
	var err error
	var db *pebble.DB
	var start time.Time
	var elapsed time.Duration
	var filename = "/tmp/pebble.db"

	//prepare db with default configuration
	os.RemoveAll(filename)
	db, err = pebble.Open(filename, nil)
	if err != nil {
		return time.Duration(0), err
	}

	//start
	start = time.Now()
	for i := 0; i < count; i++ {
		//auto-sync after insertion
		err = db.Set([]byte(strconv.Itoa(i)), data, pebble.Sync)
		if err != nil {
			return time.Duration(0), err
		}
	}
	elapsed = time.Now().Sub(start)

	err = db.Close()
	if err != nil {
		return time.Duration(0), err
	}
	os.RemoveAll(filename)
	return elapsed, nil
}

func main() {
	var pcount *int
	var punit *string
	var ptarget *string
	var unit int64
	var err error
	var data []byte
	var elapsed time.Duration

	//args for bitcask
	var pBitcaskMaxFileSize *int
	var pBitcaskMaxKeySize *uint
	var pBitcaskMaxValueSize *uint64

	//get loop count/chunk size from command argument
	pcount = flag.Int("count", 500, "loop count")
	punit = flag.String("unit", "1MB", "chunk size e.g. 1GB, 2MB, 3kb, 4b")
	ptarget = flag.String("target", "raw",
		"test target, 'raw': direct append+sync, 'rawFdatasync': preallocated file append+fdatasync, 'bolt': boltdb, 'bitcask': for bitcask, 'pebble': for pebble")
	pBitcaskMaxFileSize = flag.Int("bitcaskfilesize", 1<<20, /*1MB*/
		"max size on a signle data file, in bytes")
	pBitcaskMaxKeySize = flag.Uint("bitcaskkeysize", 64,
		"max size on key, in bytes")
	pBitcaskMaxValueSize = flag.Uint64("bitcaskvaluesize", 1<<20, /*1MB*/
		"max size on value, in bytes")

	flag.Parse()
	unit, err = ParseUnit(punit)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//default 1MB size data for test
	data = make([]byte, unit)
	randWrited, err := rand.Read(data)
	if randWrited != len(data) || err != nil {
		fmt.Println("rand.Read:", randWrited, err)
		panic("unexpected")
	}

	//start!
	fmt.Printf("loop count: %d, chunk size: %s\n\n", *pcount, *punit)
	switch *ptarget {
	case "raw":
		fmt.Println("for direct append+sync:")
		elapsed, err = TestRawOP(*pcount, data)
		if err == nil {
			PrintStats(elapsed, pcount, punit, unit)
		} else {
			fmt.Println(err)
		}
		break
	case "rawFdatasync":
		fmt.Println("preallocated file append+fdatasync:")
		elapsed, err = TestRawFdatasyncOP(*pcount, data)
		if err == nil {
			PrintStats(elapsed, pcount, punit, unit)
		} else {
			fmt.Println(err)
		}
		break
	case "bolt":
		fmt.Println("for boltdb:")
		elapsed, err = TestBolt(*pcount, data)
		if err == nil {
			PrintStats(elapsed, pcount, punit, unit)
		} else {
			fmt.Println(err)
		}
		break
	case "bitcask":
		fmt.Println("for bitcask:")
		var conf BitcaskConfig
		conf.fileSize = *pBitcaskMaxFileSize
		conf.keySize = uint32(*pBitcaskMaxKeySize)
		conf.valSize = *pBitcaskMaxValueSize
		elapsed, err = TestBitcask(*pcount, data, &conf)
		if err == nil {
			PrintStats(elapsed, pcount, punit, unit)
		} else {
			fmt.Println(err)
		}
		break
	case "pebble":
		fmt.Println("for pebble:")
		elapsed, err = TestPebble(*pcount, data)
		if err == nil {
			PrintStats(elapsed, pcount, punit, unit)
		} else {
			fmt.Println(err)
		}
		break
	default:
		fmt.Println("invalid target!")
	}
}

func PrintStats(elapsed time.Duration, pcount *int, punit *string, unit int64) {
	fmt.Printf("\ttotal time spend to append+sync all data: %s\n", elapsed.String())
	fmt.Printf("\taverage time spend to write %s: %.3fus\n",
		*punit, float64(elapsed.Microseconds())/float64(*pcount))
	fmt.Printf("\trough throughput: %.3fMB per second\n",
		float64(unit)*float64(*pcount)/float64(1024*1024)/elapsed.Seconds())
}
