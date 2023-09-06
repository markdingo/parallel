progs=para sha256sum

all: $(progs)

para:	para.go
	go build para.go

sha256sum:	sha256sum.go
	go build sha256sum.go

clean:
	rm -f $(progs)

demos: demo1 demo2 demo3 demo4

.PHONY: demo1 demo2 demo3 demo4

demo1:
	@echo demo1 - id1: stalls
	go run demo1.go >/dev/null
	@echo
	@echo No delay for id:1 and id:2 for first 10 or so lines showing "'liveliness'"
	go run demo1.go 2>/dev/null

demo2: sha256sum
	@echo
	@echo Run sha256sum in parallel with Active=6
	time ./sha256sum  -l 8 -r 800000 *.go ../*.go
	@echo
	@echo Run sha256sum serially.
	@echo The difference in "'real'" time is a function of CPU count.
	time ./sha256sum  -s -r 800000 *.go ../*.go

demo3: para
	@echo 
	@echo Use the para command to run wc -l in parallel
	./para -k wc -l ::: `ls *.go ../*.go`
	@echo
	@echo Run wc -l directly, and serially, as it should be identical.
	ls *.go ../*.go | xargs -L 1 wc -l

demo4:
	go run structfunc.go

