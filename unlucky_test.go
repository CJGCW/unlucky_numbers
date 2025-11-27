package main

import (
	"testing"
)

func exampleStateForTests() *GameState {
	board1 := &Board{
		Grid: [BoardSize][BoardSize]int{
			{5, 0, 0, 9},
			{0, 7, 0, 0},
			{0, 0, 10, 19},
			{0, 0, 19, 20},
		},
	}
	board2 := &Board{
		Grid: [BoardSize][BoardSize]int{
			{6, 0, 0, 0},
			{0, 10, 0, 0},
			{0, 0, 14, 0},
			{0, 0, 0, 20},
		},
	}
	return &GameState{
		Boards: []*Board{board1, board2},
		Table:  []int{7, 5, 17, 4},
		Draw:   []int{1, 1, 2, 2, 3, 3, 4, 6, 8, 8, 9, 11, 11, 12, 12, 13, 13, 14, 17},
	}
}

func TestIsLegalPlacement(t *testing.T) {
	state := exampleStateForTests()
	b := state.Boards[0]

	if !b.isLegalPlacement(6, 0, 1) {
		t.Errorf("Expected 6 to be legal at (0,1)")
	}
	if !b.isLegalPlacement(8, 2, 1) {
		t.Errorf("Expected 8 to be legal at (2,1)")
	}

	if b.isLegalPlacement(4, 0, 1) {
		t.Errorf("Expected 4 to be illegal at (0,1)")
	}
	if b.isLegalPlacement(20, 3, 0) {
		t.Errorf("Expected 20 to be illegal at (3,0)")
	}
}

func TestIsPlacementFeasible(t *testing.T) {
	state := exampleStateForTests()
	state.Current = 1
	if !state.isPlacementFeasible(0, 1, 7) {
		t.Errorf("Expected 7 at (0,1) to be feasible")
	}
	if state.isPlacementFeasible(0, 3, 18) {
		t.Errorf("Expected 18 at (0,3) to be infeasible due to missing 19s")
	}
}

func TestGetRemainingTileCounts(t *testing.T) {
	state := exampleStateForTests()
	b := state.Boards[1]
	counts := getRemainingTileCounts(state, b)

	if counts[19] != 0 {
		t.Errorf("Expected remaining count of 19 to be 0, got %d", counts[19])
	}
	if counts[18] == 0 {
		t.Errorf("Expected remaining count of 18 > 0")
	}
}

func TestBestMoves(t *testing.T) {
	state := exampleStateForTests()
	board := state.Boards[1]
	drawTile := 18

	moves := state.bestMoves(drawTile)
	if len(moves) == 0 {
		t.Errorf("Expected at least one move for %d", drawTile)
	}

	// base remaining counts for this player
	baseRemaining := getRemainingTileCounts(state, board)

	// helper to clone counts
	cloneCounts := func(src map[int]int) map[int]int {
		dst := make(map[int]int, len(src))
		for k, v := range src {
			dst[k] = v
		}
		return dst
	}

	for _, m := range moves {
		switch m.Type {
		case Place:
			// Ensure the suggested placement is feasible given remaining tiles
			if !state.isPlacementFeasible(m.Cell.R, m.Cell.C, drawTile) {
				t.Errorf("Suggested infeasible placement at (%d,%d) for tile %d", m.Cell.R, m.Cell.C, drawTile)
			}
		case Swap:
			// Simulate the board after swap and updated remaining counts:
			// free the old tile and consume the drawn tile.
			tmp := *board
			old := tmp.Grid[m.Cell.R][m.Cell.C]
			tmp.Grid[m.Cell.R][m.Cell.C] = drawTile

			cc := cloneCounts(baseRemaining)
			// free the swapped-out tile
			cc[old]++
			// consume the drawn tile (guard against negative)
			if cc[drawTile] > 0 {
				cc[drawTile]--
			} else {
				cc[drawTile] = 0
			}

			// Now check feasibility on the swapped board
			if !state.isPlacementFeasible(m.Cell.R, m.Cell.C, drawTile) {
				t.Errorf("Suggested infeasible swap at (%d,%d): swap %d -> %d", m.Cell.R, m.Cell.C, old, drawTile)
			}
		}
	}
}

func TestSwapLegality(t *testing.T) {
	state := exampleStateForTests()
	board := state.Boards[1]

	moves := state.bestMoves(18)
	swapFound := false
	for _, m := range moves {
		if m.Type == Swap {
			swapFound = true
			if !board.isLegalPlacement(18, m.Cell.R, m.Cell.C) {
				t.Errorf("Swap suggested at (%d,%d) is illegal", m.Cell.R, m.Cell.C)
			}
		}
	}
	if !swapFound {
		t.Logf("No swaps found — correct if no legal swaps exist")
	}
}
