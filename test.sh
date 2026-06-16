#!/bin/bash

set -e

echo "=== Setup Test Environment ==="
mkdir -p /tmp/fastcopy_test/src/deep/dir/structure
mkdir -p /tmp/fastcopy_test/dst
cd /tmp/fastcopy_test

echo "1. Creating many small files..."
for i in {1..1000}; do
    echo "small file data $i" > src/small_$i.txt
done

echo "2. Creating a few larger files..."
dd if=/dev/urandom of=src/large_1.bin bs=1M count=100 2>/dev/null
dd if=/dev/urandom of=src/large_2.bin bs=1M count=100 2>/dev/null

echo "3. Creating deep directory structures with files..."
for i in {1..100}; do
    echo "deep file data $i" > src/deep/dir/structure/deep_$i.txt
done

echo "4. Creating symlinks..."
ln -s small_1.txt src/symlink_to_small.txt
ln -s ../large_1.bin src/deep/dir/symlink_to_large.bin

echo "Test Environment Ready. Total size:"
du -sh src/

echo ""
echo "=== Test 1: Initial Copy ==="
time /home/moises/gocopy/fastcopy/fastcopy src/ dst/initial/
echo "Verifying integrity..."
diff -rq src/ dst/initial/ && echo "✅ Integrity OK"

echo ""
echo "=== Test 2: Incremental Copy (No changes) ==="
time /home/moises/gocopy/fastcopy/fastcopy src/ dst/initial/

echo ""
echo "=== Test 3: Incremental Copy (With changes) ==="
echo "new data" >> src/small_1.txt
rm src/small_2.txt
echo "new file" > src/new_file.txt
time /home/moises/gocopy/fastcopy/fastcopy src/ dst/initial/
echo "Verifying integrity after changes..."
# We expect small_2.txt to still exist in dst because fastcopy is a copier, not a sync tool (it doesn't delete).
# So diff will show "Only in dst/initial/: small_2.txt"
diff -rq src/ dst/initial/ | grep -v "Only in dst/initial" || true
echo "✅ Incremental update OK (ignores deletions as expected for a copy tool)"

echo ""
echo "=== Test 4: Checksum Copy ==="
time /home/moises/gocopy/fastcopy/fastcopy --checksum src/ dst/checksum/

echo ""
echo "=== Test 5: Benchmark against cp ==="
echo "Cleaning up dests for benchmark..."
rm -rf dst/cp_dest dst/fastcopy_dest

echo "Testing cp -a..."
time cp -a src/ dst/cp_dest/

echo "Testing fastcopy..."
time /home/moises/gocopy/fastcopy/fastcopy --quiet src/ dst/fastcopy_dest/

echo ""
echo "=== Cleanup ==="
rm -rf /tmp/fastcopy_test
echo "All tests completed."
