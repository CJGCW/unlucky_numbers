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

func TestIsPlacementFeasible(t *testing.T) {
	state := exampleStateForTests()

	if !state.isPlacementFeasible(6, 0, 1) {
		t.Errorf("Expected 6 to be legal at (0,1)")
	}
	if !state.isPlacementFeasible(8, 2, 1) {
		t.Errorf("Expected 8 to be legal at (2,1)")
	}
	if state.isPlacementFeasible(4, 0, 1) {
		t.Errorf("Expected 4 to be illegal at (0,1)")
	}
	if state.isPlacementFeasible(20, 3, 0) {
		t.Errorf("Expected 20 to be illegal at (3,0)")
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

			// Now check feasibility on the swapped board
			if !state.isPlacementFeasible(drawTile, m.Cell.R, m.Cell.C) {
				t.Errorf("Suggested infeasible swap at (%d,%d): swap %d -> %d", m.Cell.R, m.Cell.C, old, drawTile)
			}
		}
	}
}

func TestSwapLegality(t *testing.T) {
	state := exampleStateForTests()
	state.Current = 1

	moves := state.bestMoves(18)
	swapFound := false
	for _, m := range moves {
		if m.Type == Swap {
			swapFound = true
			if !state.isPlacementFeasible(18, m.Cell.R, m.Cell.C) {
				t.Errorf("Swap suggested at (%d,%d) is illegal", m.Cell.R, m.Cell.C)
			}
		}
	}
	if !swapFound {
		t.Logf("No swaps found — correct if no legal swaps exist")
	}
}
