N [alive=="true"] {
	clone($O, $);
}

E[head.alive=="true" && tail.alive=="true"] {
	clone($O,$);
}
