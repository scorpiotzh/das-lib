package example

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/DeAccountSystems/das-lib/common"
	"github.com/DeAccountSystems/das-lib/core"
	"github.com/DeAccountSystems/das-lib/sign"
	"testing"
)

func TestMixin(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		println(err)
		return
	}
	fmt.Println(common.Bytes2Hex(privateKey))
	fmt.Println(common.Bytes2Hex(publicKey), len(common.Bytes2Hex(publicKey)), len(publicKey))
	msg := common.Hex2Bytes("0x6d31bda56835b9c2d4876a53d611dfb58238aacf26ca00d3c8f5a2165c3f70cf")
	res := ed25519.Sign(privateKey, msg)
	fmt.Println(common.Bytes2Hex(res))
	fmt.Println(ed25519.Verify(publicKey, msg, res))

	// 0x70c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec17 66 32
	//args1 := "0x0515a33588908cF8Edb27D1AbE3852Bf287Abd38910515a33588908cF8Edb27D1AbE3852Bf287Abd3891"
	//args2 := "0x0670c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec170670c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec17"
	//fmt.Println(len(args1), len(args2))
	//fmt.Println(len(common.Hex2Bytes(args1)), len(common.Hex2Bytes(args2)))
}

func TestFormatDasLockToOwnerAndManager(t *testing.T) {
	//args := "0x0515a33588908cF8Edb27D1AbE3852Bf287Abd38910515a33588908cF8Edb27D1AbE3852Bf287Abd3891"
	//args := "0x0670c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec170670c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec17"
	//args:="0x0670c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec170515a33588908cF8Edb27D1AbE3852Bf287Abd3891"
	args := "0x0515a33588908cf8edb27d1abe3852bf287abd38910670c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec17"
	owner, manager := core.FormatDasLockToOwnerAndManager(common.Hex2Bytes(args))
	fmt.Println(common.Bytes2Hex(owner), common.Bytes2Hex(manager))
}

func TestFormatAddressToHex(t *testing.T) {
	fmt.Println(core.FormatAddressToHex(common.ChainTypeCkb, "ckb1qjfhdsa4syv599s2s3nfrctwga70g0tu07n9gpnun9ydlngf5vsnwqeerx5wkcvuet3jlw5g6vec9xffmvh5xfqr8yv636mpnn9wxta63rfn8q5e98dj7sey04pzkp"))
}

func TestFormatOwnerManagerAddressToArgs2(t *testing.T) {
	oCT, mCT, oA, mA := common.ChainTypeEth, common.ChainTypeTron, "0xc9f53b1d85356B60453F867610888D89a0B667Ad", "TEooRfPxhqJ7AJfmsRg5hZWEX95VeNxvtX"
	args := core.FormatOwnerManagerAddressToArgs(oCT, mCT, oA, mA)
	fmt.Println(common.Bytes2Hex(args))
	oCT, oA = common.ChainTypeMixin, "0x70c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec17"
	args = core.FormatOwnerManagerAddressToArgs(oCT, mCT, oA, mA)
	fmt.Println(common.Bytes2Hex(args))
	mCT, mA = common.ChainTypeMixin, "0x70c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec17"
	args = core.FormatOwnerManagerAddressToArgs(oCT, mCT, oA, mA)
	fmt.Println(common.Bytes2Hex(args))
}

func TestFormatDasLockToHexAddress(t *testing.T) {
	args := "0x053919a8eb619ccae32fba88d333829929db2f432405c9f53b1d85356b60453f867610888d89a0b667ad"
	fmt.Println(core.FormatDasLockToHexAddress(common.Hex2Bytes(args)))
	args = "0x0670c756ecfa897dc71cbfce48931fbb261f2e593fe234902a57f36aa9c27dec170515a33588908cF8Edb27D1AbE3852Bf287Abd3891"
	fmt.Println(core.FormatDasLockToHexAddress(common.Hex2Bytes(args)))
}

func TestEd25519Signature(t *testing.T) {
	private := common.Hex2Bytes("0xf2011f49d9ad51fe64ce0f03afcff509e0324a046d8ef9b509805678fd2d9254e1090ce82474cbe0b196d1e62ec349ec05a61076c68d14129265370ca7e051c4")
	msg := common.Hex2Bytes("0x01d4a002a560595c5e6f828c78eb7aafae23544f1c48263b76a0be3bfce219bd")
	sin := sign.Ed25519Signature(private, msg)
	fmt.Println(common.Bytes2Hex(sin))
	//sin = common.Hex2Bytes("0x4d0fff8474b060546d7cd5310ba317412e100c53bad5a15665052e344b615f979bf951666276d8e1548d2a39b899f518d2d6718ad4bfe4ee2b6bc988b049bd0d")
	pub := common.Hex2Bytes("0xe1090ce82474cbe0b196d1e62ec349ec05a61076c68d14129265370ca7e051c4")
	fmt.Println(sign.VerifyEd25519Signature(pub, msg, sin))
}
