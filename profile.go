package main

import (
	"context"
	"encoding/json"
	"fmt"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"os"
)

func doShowProfile(cCtx *cli.Context) error {
	if cCtx.Args().Present() {
		return cli.ShowSubcommandHelp(cCtx)
	}

	xrpcc, err := makeXRPCC(cCtx)
	if err != nil {
		return fmt.Errorf("cannot create client: %w", err)
	}

	arg := cCtx.String("handle")
	if arg == "" {
		arg = xrpcc.Auth.Handle
	}

	profile, err := bsky.ActorGetProfile(context.TODO(), xrpcc, arg)
	if err != nil {
		return fmt.Errorf("cannot get profile: %w", err)
	}

	if cCtx.Bool("json") {
		json.NewEncoder(os.Stdout).Encode(profile)
		return nil
	}

	fmt.Printf("Did: %s\n", profile.Did)
	fmt.Printf("Handle: %s\n", profile.Handle)
	fmt.Printf("DisplayName: %s\n", stringp(profile.DisplayName))
	fmt.Printf("Description: %s\n", stringp(profile.Description))
	fmt.Printf("Follows: %d\n", int64p(profile.FollowsCount))
	fmt.Printf("Followers: %d\n", int64p(profile.FollowersCount))
	fmt.Printf("Avatar: %s\n", stringp(profile.Avatar))
	fmt.Printf("Banner: %s\n", stringp(profile.Banner))
	return nil
}

func doLogin(cCtx *cli.Context) error {
	fp, _ := cCtx.App.Metadata["path"].(string)
	var cfg config
	cfg.Host = cCtx.String("host")
	cfg.Handle = cCtx.Args().Get(0)
	cfg.Password = cCtx.Args().Get(1)
	if cfg.Handle == "" || cfg.Password == "" {
		cli.ShowSubcommandHelpAndExit(cCtx, 1)
	}
	b, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot make config file: %w", err)
	}
	err = ioutil.WriteFile(fp, b, 0644)
	if err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}
	return nil
}

func doShowSession(cCtx *cli.Context) error {
	xrpcc, err := makeXRPCC(cCtx)
	if err != nil {
		return fmt.Errorf("cannot create client: %w", err)
	}

	session, err := comatproto.ServerGetSession(context.TODO(), xrpcc)
	if err != nil {
		return err
	}

	if cCtx.Bool("json") {
		json.NewEncoder(os.Stdout).Encode(session)
		return nil
	}

	fmt.Printf("Did: %s\n", session.Did)
	fmt.Printf("Email: %s\n", stringp(session.Email))
	fmt.Printf("Handle: %s\n", session.Handle)
	return nil
}
