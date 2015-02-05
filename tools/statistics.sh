#!/bin/sh

usage() {
	exec >&2
	echo "Usage:"
	echo "  $0 <dotfile>"
	exit 1
}

top5() {
	gvpr "$1" "$2" | sort | uniq -c | sort -rn | head -n 5 | \
		awk '{ printf "  .%-5s %d\n", $2, $1}'

}

[ $# -eq 1 ] || usage
[ -f "$1" ] || usage

echo "Gopherholes"
gvpr '
	BEGIN{ int a = 0; int d = 0; }
	N[alive==""] { d++; }
	N[alive=="true"] { a++; }
	END {
		printf("  alive:  %d\n", a);
		printf("  dead:   %d\n", d);
		printf("  total:  %d\n", a+d);
	}
' "$1"

echo
echo "Top 5 TLDs for alive gopherholes" 
top5 '
	N[alive=="true"] {
		int i = rindex($.name, ":");
		string s = substr($.name, 0, i);
		i = rindex($.name, ".");
		s = substr(s, i+1);
		print(s);
	}
' "$1"

echo
echo "Top 5 TLDs for dead gopherholes" 
top5 '
	N[alive==""] {
		int i = rindex($.name, ":");
		string s = substr($.name, 0, i);
		i = rindex($.name, ".");
		s = substr(s, i+1);
		print(s);
	}
' "$1"
