package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"p2poker/internal/cluster"
	"p2poker/internal/netx"
	"p2poker/internal/protocol"
	"p2poker/pkg/types"
)

func main() {
	listen := flag.String("listen", ":7777", "tcp listen addr")
	peer := flag.String("peer", "", "peer addr to dial (optional)")
	inproc := flag.Bool("inproc", false, "use in-process loopback network (for single-process demos)")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var nw netx.Network
	if *inproc {
		nw = netx.NewInproc()
	} else {
		nw = netx.NewTCP(*listen)
	}

	n := cluster.NewNode(*listen, nw)
	if err := n.Start(ctx); err != nil {
		panic(err)
	}

	if *peer != "" {
		if tcp, ok := n.Network().(*netx.TCP); ok {
			if err := tcp.AddPeer(*peer); err != nil {
				fmt.Println("dial error:", err)
			}
		} else {
			fmt.Println("peer flag ignored (not running TCP mode)")
		}
	}

	fmt.Printf("node: %s listening on %s", n.ID, *listen)
	fmt.Println("type 'help' for commands")
	repl(ctx, n)
}

func repl(ctx context.Context, n *cluster.Node) {
	s := bufio.NewScanner(os.Stdin)
	prompt := func() { fmt.Print("> ") }
	prompt()
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			prompt()
			continue
		}
		args := strings.Fields(line)
		switch strings.ToLower(args[0]) {
		case "help":
			printHelp()
		case "whoami":
			fmt.Println("node:", n.ID)
		case "create":
			name := "Table"
			sb, bb, min := int64(5), int64(10), int64(200)
			if len(args) > 1 {
				name = args[1]
			}
			if len(args) > 2 {
				sb = mustI64(args[2])
			}
			if len(args) > 3 {
				bb = mustI64(args[3])
			}
			if len(args) > 4 {
				min = mustI64(args[4])
			}
			id, err := n.CreateTable(name, sb, bb, min)
			if err != nil {
				fmt.Println("error:", err)
			} else {
				fmt.Println("created:", id)
			}
		case "tables":
			// tables [-v]
			if len(args) > 1 && args[1] == "-v" {
				list := n.Manager().ListVerbose(n.ID)
				if len(list) == 0 {
					fmt.Println("(no tables)")
				} else {
					for _, it := range list {
						fmt.Printf("- %s epoch=%d authority=%s is_authority=%v", it.ID, it.Epoch, it.Authority, it.IsAuthority)
					}
				}
			} else {
				ids := n.Manager().ListIDs()
				if len(ids) == 0 {
					fmt.Println("(no tables)")
				} else {
					for _, id := range ids {
						fmt.Println("-", id)
					}
				}
			}
		case "discover":
			// discover <tableID>
			if len(args) < 2 {
				fmt.Println("usage: discover <tableID>")
				break
			}
			tid := protocol.TableID(args[1])
			if id, err := n.DiscoverAndAttach(tid); err != nil {
				fmt.Println("discover error:", err)
			} else {
				fmt.Println("discovered and attached:", id)
			}
		case "attach":
			if len(args) < 7 {
				fmt.Println("usage: attach <tableID> <name> <sb> <bb> <min> <epoch>")
				break
			}
			tid := protocol.TableID(args[1])
			cfg := types.TableConfig{Name: args[2], SmallBlind: mustI64(args[3]), BigBlind: mustI64(args[4]), MinBuyin: mustI64(args[5])}
			epoch := protocol.Epoch(mustU64(args[6]))
			if err := n.JoinTableRemote(tid, epoch, cfg); err != nil {
				fmt.Println("error:", err)
			} else {
				fmt.Println("attached follower to:", tid)
			}
		case "join":
			if len(args) < 2 {
				fmt.Println("usage: join <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActJoin, PlayerID: string(n.ID)})
				fmt.Println("join proposed on", id)
			} else {
				fmt.Println("unknown table locally; try 'discover <id>'")
			}
		case "leave":
			// leave <tableID>
			if len(args) < 2 {
				fmt.Println("usage: leave <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActLeave, PlayerID: string(n.ID)})
				fmt.Println("leave proposed on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "kick":
			// kick <tableID> <playerNodeID>
			if len(args) < 3 {
				fmt.Println("usage: kick <tableID> <playerNodeID>")
				break
			}
			id := protocol.TableID(args[1])
			target := args[2]
			if t, ok := n.Manager().Get(id); ok {
				ss := t.Snapshot()
				if ss.Authority != n.ID {
					fmt.Println("you are not the authority; cannot kick")
					break
				}
				meta := map[string]any{"target": target}
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActKick, PlayerID: string(n.ID), Meta: meta})
				fmt.Println("kick proposed:", target, "on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "bet":
			if len(args) < 3 {
				fmt.Println("usage: bet <tableID> <amount>")
				break
			}
			id := protocol.TableID(args[1])
			amt := mustI64(args[2])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActBet, PlayerID: string(n.ID), Amount: amt})
				fmt.Println("bet proposed:", amt, "on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "check":
			// check <tableID>
			if len(args) < 2 {
				fmt.Println("usage: check <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActCheck, PlayerID: string(n.ID)})
				fmt.Println("check proposed on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "fold":
			// fold <tableID>
			if len(args) < 2 {
				fmt.Println("usage: fold <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActFold, PlayerID: string(n.ID)})
				fmt.Println("fold proposed on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "call":
			// call <tableID>
			if len(args) < 2 {
				fmt.Println("usage: call <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActCall, PlayerID: string(n.ID)})
				fmt.Println("call proposed on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "raise":
			// raise <tableID> <amount>
			if len(args) < 3 {
				fmt.Println("usage: raise <tableID> <amount>")
				break
			}
			id := protocol.TableID(args[1])
			amt := mustI64(args[2])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActRaise, PlayerID: string(n.ID), Amount: amt})
				fmt.Println("raise proposed:", amt, "on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "state":
			// state [-v] <tableID>
			if len(args) < 2 {
				fmt.Println("usage: state [-v] <tableID>")
				break
			}
			verbose := false
			tidIdx := 1
			if args[1] == "-v" {
				verbose = true
				if len(args) < 3 {
					fmt.Println("usage: state -v <tableID>")
					break
				}
				tidIdx = 2
			}
			id := protocol.TableID(args[tidIdx])
			if t, ok := n.Manager().Get(id); ok {
				ss := t.Snapshot()
				fmt.Printf("table=%s epoch=%d seq=%d auth=%s cfg={SB=%d BB=%d}\n",
					id, ss.Epoch, ss.Seq, ss.Authority, ss.Cfg.SmallBlind, ss.Cfg.BigBlind)

				// Pull live engine summary for nicer view
				summary := t.Eng().Summary()

				fmt.Printf("phase=%s pot=%d dealer=%s turn=%s\n",
					summary.Phase, summary.Pot, summary.Dealer, summary.Turn)

				if verbose {
					fmt.Println("seats:")
					for _, sv := range summary.Seats {
						marks := ""
						if sv.Player == summary.Turn {
							marks += " ←turn"
						}
						if sv.Player == summary.Dealer {
							if marks != "" {
								marks += ", "
							}
							marks += "dealer"
						}

						flags := ""
						if sv.Folded {
							flags += " folded"
						}
						if sv.AllIn {
							flags += " all-in"
						}
						if sv.InHand && !sv.Folded {
							flags += " in-hand"
						}
						if flags != "" {
							flags = " [" + strings.TrimSpace(flags) + "]"
						}

						fmt.Printf(" - %s stack=%d committed=%d%s%s\n",
							sv.Player, sv.Stack, sv.Committed, flags, marks)
					}
				} else {
					fmt.Println("(use 'state -v <tableID>' for stacks/flags)")
				}
			} else {
				fmt.Println("unknown table")
			}
		case "start":
			if len(args) < 2 {
				fmt.Println("usage: start <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActStartHand, PlayerID: string(n.ID)})
				fmt.Println("hand start proposed on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "advance":
			if len(args) < 2 {
				fmt.Println("usage: advance <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActAdvance, PlayerID: string(n.ID)})
				fmt.Println("advance proposed on", id)
			} else {
				fmt.Println("unknown table")
			}
		case "snapshot":
			if len(args) < 2 {
				fmt.Println("usage: snapshot <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				ss := t.Snapshot()
				fmt.Printf("table %s epoch=%d seq=%d authority=%s cfg={%s SB=%d BB=%d}", id, ss.Epoch, ss.Seq, ss.Authority, ss.Cfg.Name, ss.Cfg.SmallBlind, ss.Cfg.BigBlind)
			} else {
				fmt.Println("unknown table")
			}
		case "epoch":
			// epoch <tableID>
			if len(args) < 2 {
				fmt.Println("usage: epoch <tableID>")
				break
			}
			id := protocol.TableID(args[1])
			if t, ok := n.Manager().Get(id); ok {
				ss := t.Snapshot()
				isAuth := ss.Authority == n.ID
				fmt.Printf("epoch=%d authority=%s is_authority=%v", ss.Epoch, ss.Authority, isAuth)
			} else {
				fmt.Println("unknown table")
			}
		case "addpeer":
			if len(args) < 2 {
				fmt.Println("usage: addpeer <addr>")
				break
			}
			if tcp, ok := n.Network().(*netx.TCP); ok {
				if err := tcp.AddPeer(args[1]); err != nil {
					fmt.Println("dial error:", err)
				} else {
					fmt.Println("peer added")
				}
			} else {
				fmt.Println("addpeer only supported in TCP mode")
			}
		case "quit", "exit":
			fmt.Println("bye")
			return
		default:
			fmt.Println("unknown command; type 'help'")
		}
		prompt()
	}
}

func printHelp() {
	fmt.Println(`commands:
  whoami
  create <name> [sb bb min]
	tables
  discover <tableID>
  attach <tableID> <name> <sb> <bb> <min> <epoch>
  join <tableID>
	leave <tableID>
	kick <tableID> <playerNodeID>
  bet <tableID> <amount>
	check <tableID>
  fold <tableID>
	call <tableID>
  raise <tableID> <amount>
  state <tableID>
  start <tableID>
  advance <tableID>
  snapshot <tableID>
  epoch <tableID>
  addpeer <addr>
  quit`)
}

func mustI64(s string) int64  { v, _ := strconv.ParseInt(s, 10, 64); return v }
func mustU64(s string) uint64 { v, _ := strconv.ParseUint(s, 10, 64); return v }
