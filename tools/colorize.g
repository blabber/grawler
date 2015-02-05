BEGIN {
	string alivefillcolor = "#0000ff00";
	string alivecolor = "#000000ff";
	string deadfillcolor = "#ff00007f";
	string deadcolor = "#ff0000";
}

N {
	style = "filled";
	fillcolor = deadfillcolor;
	color = deadcolor;
}

N [alive=="true"] {
	style = "filled";
	fillcolor = alivefillcolor;
	color = alivecolor;
}

E {
	color = deadcolor;
}

E [head.alive=="true" && tail.alive=="true"] {
	color = alivecolor;
}

END_G {
	$O = $G;
}
