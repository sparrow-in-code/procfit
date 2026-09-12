package query

// applyHaving filters rows recursively by a RowPredicate (the `having` stage).
// A group is kept if it passes the predicate OR still has surviving children, so
// a matching leaf keeps its ancestors visible. A nil predicate passes all.
func applyHaving(rows []*Row, pred RowPredicate) []*Row {
	if pred == nil {
		return rows
	}
	out := make([]*Row, 0, len(rows))
	for _, r := range rows {
		r.Sub = applyHaving(r.Sub, pred)
		keep := pred.EvalRow(r)
		if keep || len(r.Sub) > 0 {
			out = append(out, r)
		}
	}
	return out
}
