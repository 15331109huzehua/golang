package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
)

//  build struct
type selpg_args struct {
	start_page  int
	end_page    int
	in_filename string
	page_len    int
	page_type   int
	print_dest  string
}

const INT_MAX int = 1<<32 - 1
const LINE_SIZE int = 1024
const INBUFSIZ int = 16 * 1024

var proname string

// get project name
func setname(name string) string {
	var pos = 0
	for i, ch := range name {
		if ch == '/' {
			pos = i
		}
	}
	//get porject name
	return name[pos:]
}

// main
func main() {
	proname = setname(os.Args[0])
	// default
	var sa selpg_args
	sa.start_page = -1
	sa.end_page = -1
	sa.in_filename = ""
	sa.page_len = 72
	sa.page_type = 'l'
	sa.print_dest = ""

	process_args(&sa)
	process_input(sa)
}

func process_args(sa *selpg_args) {
	//不可以少于三个命令。
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "%s: not enough arguments\n",
			proname)
		usage()
		os.Exit(1)
	}
	//用于获取参数
	var s string
	//用于参数赋值
	var i int

	/* handle 1st arg - start page */
	s = os.Args[1]
	//获取开始页
	if len(s) < 2 || s[:2] != "-s" {
		fmt.Fprintf(os.Stderr, "%s: 1st arg should be -sstart_page\n",
			proname)
		usage()
		os.Exit(2)
	}
	//char转int 获取数值
	i, _ = strconv.Atoi(s[2:])
	//判断数字是否合法
	if i < 1 || i > INT_MAX {
		fmt.Fprintf(os.Stderr, "%s: invalid start page %s\n",
			proname, s[2:])
		usage()
		os.Exit(3)
	}
	//赋值
	sa.start_page = i

	/* handle 2nd arg - end page */
	s = os.Args[2]
	//获取结束页
	if len(s) < 2 || s[:2] != "-e" {
		fmt.Fprintf(os.Stderr, "%s: 2nd arg should be -eend_page\n",
			proname)
		usage()
		os.Exit(4)
	}
	//char转int 获取数值
	i, _ = strconv.Atoi(s[2:])
	//判断数字是否合法
	if i < 1 || i > INT_MAX {
		fmt.Fprintf(os.Stderr, "%s: invalid end page %s\n",
			proname, s[2:])
		usage()
		os.Exit(5)
	}
	//赋值
	sa.end_page = i
	/* now handle optional args */
	argnum := 3 //用于infile时的比较
	//循环获得之后参数
	for _, s = range os.Args[3:] {
		//第一个字符不合法直接退出
		if s[0] != '-' {
			break
		}
		argnum++
		//通过switch进行判断
		switch s[1] {
		case 'f':
			if s != "-f" {
				fmt.Fprintf(os.Stderr, "%s: option should be \"-f\"\n", proname)
				usage()
				os.Exit(7)
			}
			//成功的话赋值
			sa.page_type = 'f'
		case 'l':
			//获取之后的参数
			i, _ = strconv.Atoi(s[2:])
			//判断是否合法
			if i < 1 || i > INT_MAX {
				fmt.Fprintf(os.Stderr, "%s: invalid page length %s\n", proname, s[2:])
				usage()
				os.Exit(6)
			}
			sa.page_len = i
		case 'd':
			if s == "-d" {
				fmt.Fprintf(os.Stderr, "%s: -d option requires a printer destination\n", proname)
				usage()
				os.Exit(8)
			}
			sa.print_dest = s[2:]
		//当都不满足时输出错误
		default:
			fmt.Fprintf(os.Stderr, "%s: unknown option %s\n", proname, s)
			usage()
			os.Exit(9)
		}
	}

	// handle input_file
	if argnum < len(os.Args) {
		s = os.Args[argnum]
		//把值赋给filename
		sa.in_filename = s
		//如果出错或文件不存在，输出错误
		if _, err := os.Stat(s); err != nil && os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s: input file \"%s\" does not exist\n", proname, s)
			os.Exit(10)
		}
	}
}

func process_input(sa selpg_args) {
	var err error
	var s string
	var page int        //记录行数
	var line int        //记录页数
	var r *bufio.Reader //记录读取的缓存
	var w *bufio.Writer //记录写的缓存
	var cmd *exec.Cmd   //用于命令行的读写
	var file *os.File   //文件

	/* set the input source */
	if sa.in_filename == "" {
		//若filenam为空，则取标准输入
		r = bufio.NewReaderSize(os.Stdin, INBUFSIZ)
	} else {
		//读取文件，并存入缓存区
		file, err = os.OpenFile(sa.in_filename, os.O_RDONLY, 0)
		r = bufio.NewReaderSize(file, INBUFSIZ)
		// cannot open file
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s: could not open input file \"%s\"\n", proname, sa.in_filename)
			os.Exit(11)
		}
	}

	/* set the output destination */
	if sa.print_dest == "" {
		//若打印机为空，则标准输出与屏幕
		w = bufio.NewWriterSize(os.Stdout, INBUFSIZ)
	} else {
		//调用命令行启动打印机
		cmd = exec.Command("lp", "-d"+sa.print_dest)
		//从管道中得到文件流
		fout, err := cmd.StdinPipe()
		//将其存入写缓冲区
		w = bufio.NewWriterSize(fout, INBUFSIZ)
		// cannot open pipe
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not open pipe to \"%s\"\n", proname, "ls "+s)
			os.Exit(12)
		}
	}

	/* begin one of two main loops based on page type */
	if sa.page_type == 'l' {
		line = 0
		page = 1
		//无错误循环
		for true {
			//创建一个LINE_SIZE*byte[]大小的流
			crc := make([]byte, LINE_SIZE)
			//read in
			_, err = r.Read(crc)

			// error or EOF
			if err != nil {
				if err == io.EOF {
					break
				} else {
					fmt.Fprintf(os.Stderr, "%s: input stream error\n", proname)
				}
			}
			line++
			//换页
			if line > sa.page_len {
				page++
				line = 1
			}
			// 写入写缓冲区
			if page >= sa.start_page && page <= sa.end_page {
				w.WriteString(string(crc))
			}
		}
	} else {
		page = 1

		for true {
			//按字节读入
			ch, _, err := r.ReadRune()
			// error or EOF
			if err != nil {
				if err == io.EOF {
					break
				} else {
					fmt.Fprintf(os.Stderr, "%s: input stream error\n", proname)
				}
			}
			//如果读到\f说明到了页尾，需要换页
			if ch == '\f' {
				page++
			}
			//按字节写入写缓冲区
			if page >= sa.start_page && page <= sa.end_page {
				w.WriteRune(ch)
			}
		}
	}
	//冲掉缓存
	r.Flush()
	w.Flush()

	// 输出结束
	//页范围判断
	if page < sa.start_page {
		fmt.Fprintf(os.Stderr, "%s: start_page (%d) greater than total pages (%d),"+
			" no output written\n", proname, sa.start_page, page)
	} else if page < sa.end_page {
		fmt.Fprintf(os.Stderr, "%s: end_page (%d) greater than total pages (%d),"+
			" less output than expected\n", proname, sa.end_page, page)
	} else {
		//关闭file
		if sa.in_filename != "" {
			file.Close()
		}
		//当打印结束时，把文件流输出到屏幕
		if sa.print_dest != "" {
			//when lp end ,printf output.
			out, _ := cmd.CombinedOutput()
			fmt.Fprint(os.Stderr, string(out))
		}
		// finish
		fmt.Fprintf(os.Stderr, "\n%s: done\n", proname)
	}
}

// 正确格式提示
func usage() {
	fmt.Println("\nUSAGE: %d -sstart_page -eend_page [ -f | -llines_per_page ]"+
		" [ -ddest ] [ in_filename ]", proname)
}
