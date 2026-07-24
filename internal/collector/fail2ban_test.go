package collector

import (
	"reflect"
	"testing"

	"github.com/rtf6x/mogotor/internal/models"
)

func TestParseFail2banJailList(t *testing.T) {
	input := "Status\n|- Number of jail:\t2\n`- Jail list:\tsshd, recidive\n"
	jails := parseFail2banJailList(input)
	want := []string{"sshd", "recidive"}
	if !reflect.DeepEqual(jails, want) {
		t.Fatalf("got %#v, want %#v", jails, want)
	}
}

func TestParseFail2banJailStatus(t *testing.T) {
	input := "Status for the jail: sshd\n" +
		"|- Filter\n" +
		"|  |- Currently failed:\t2\n" +
		"|  |- Total failed:\t123\n" +
		"|  `- File list:\t/var/log/auth.log\n" +
		"`- Actions\n" +
		"   |- Currently banned:\t3\n" +
		"   |- Total banned:\t40\n" +
		"   `- Banned IP list:\t1.2.3.4 5.6.7.8 9.9.9.9\n"
	jail := parseFail2banJailStatus("sshd", input)
	if jail.Name != "sshd" {
		t.Fatalf("name: got %q", jail.Name)
	}
	if jail.CurrentlyFailed != 2 || jail.TotalFailed != 123 {
		t.Fatalf("failed counts: %+v", jail)
	}
	if jail.CurrentlyBanned != 3 || jail.TotalBanned != 40 {
		t.Fatalf("banned counts: %+v", jail)
	}
	wantIPs := []string{"1.2.3.4", "5.6.7.8", "9.9.9.9"}
	if !reflect.DeepEqual(jail.BannedIPs, wantIPs) {
		t.Fatalf("banned IPs: got %#v, want %#v", jail.BannedIPs, wantIPs)
	}
}

func TestParseFail2banJailStatusEmptyBans(t *testing.T) {
	input := "Status for the jail: sshd\n" +
		"|- Filter\n" +
		"|  |- Currently failed:\t0\n" +
		"|  |- Total failed:\t10\n" +
		"|  `- File list:\t/var/log/auth.log\n" +
		"`- Actions\n" +
		"   |- Currently banned:\t0\n" +
		"   |- Total banned:\t5\n" +
		"   `- Banned IP list:\n"
	jail := parseFail2banJailStatus("sshd", input)
	if jail.CurrentlyBanned != 0 || len(jail.BannedIPs) != 0 {
		t.Fatalf("expected empty bans, got %+v", jail)
	}
}

func TestParseFail2banLogCurrentlyBanned(t *testing.T) {
	input := "2026-07-22 10:00:01,000 fail2ban.actions [1]: NOTICE  [sshd] Ban 1.2.3.4\n" +
		"2026-07-22 10:05:00,000 fail2ban.actions [1]: NOTICE  [sshd] Ban 5.6.7.8\n" +
		"2026-07-22 10:10:00,000 fail2ban.actions [1]: NOTICE  [sshd] Unban 1.2.3.4\n" +
		"2026-07-22 10:15:00,000 fail2ban.actions [1]: NOTICE  [recidive] Ban 9.9.9.9\n"
	jails := parseFail2banLogCurrentlyBanned(input)
	byName := map[string]models.Fail2banJail{}
	for _, jail := range jails {
		byName[jail.Name] = jail
	}

	sshd, ok := byName["sshd"]
	if !ok || !reflect.DeepEqual(sshd.BannedIPs, []string{"5.6.7.8"}) {
		t.Fatalf("sshd bans: %#v", sshd)
	}
	if sshd.CurrentlyBanned != 1 {
		t.Fatalf("sshd currently banned: %d", sshd.CurrentlyBanned)
	}

	recidive, ok := byName["recidive"]
	if !ok || !reflect.DeepEqual(recidive.BannedIPs, []string{"9.9.9.9"}) {
		t.Fatalf("recidive bans: %#v", recidive)
	}
}
