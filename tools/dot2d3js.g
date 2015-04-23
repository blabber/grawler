BEGIN {
	string nodes = "";
	int firstnode = 1;
	string nodeseperator = "";
	string links = "";
	int firstlink = 1;
	string linkseperator = "";
}

N [alive == "true"] {
	nodes = sprintf("%s%s{\"name\":\"%s\", \"alive\":%d}",
		nodes, nodeseperator, $.name, 1);

	if (firstnode) {
		firstnode = 0;
		nodeseperator = ",";
	}
}

N [alive == ""] {
	nodes = sprintf("%s%s{\"name\":\"%s\", \"alive\":%d}",
		nodes, nodeseperator, $.name, 0);

	if (firstnode) {
		firstnode = 0;
		nodeseperator = ",";
	}
}

E  {
	if (strcmp($.head.name, $.tail.name) == 0) {
		return;
	}

	links = sprintf("%s%s{\"source\":\"%s\", \"target\":\"%s\"}",
		links, linkseperator, $.head.name, $.tail.name);

	if (firstlink) {
		firstlink = 0;
		linkseperator = ",";
	}
}

END {
	printf("{\"nodes\": [%s], \"links\": [%s]}", nodes, links);
}
