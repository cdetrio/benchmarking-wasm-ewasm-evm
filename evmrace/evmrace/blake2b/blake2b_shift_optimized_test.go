package runtime

import (
	"strings"
	"testing"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)


/*
// based on https://github.com/ConsenSys/Project-Alchemy/blob/master/contracts/BLAKE2b/BLAKE2b.sol

// compiled with solc version:0.5.4+commit.9549d8ff.Emscripten.clang with optimizer enabled
// hand optimized to replace div and mul with shr and shl

pragma solidity ^0.5.1;

contract BLAKE2b {

  uint64 constant MASK_0 = 0xFF00000000000000;
  uint64 constant MASK_1 = 0x00FF000000000000;
  uint64 constant MASK_2 = 0x0000FF0000000000;
  uint64 constant MASK_3 = 0x000000FF00000000;
  uint64 constant MASK_4 = 0x00000000FF000000;
  uint64 constant MASK_5 = 0x0000000000FF0000;
  uint64 constant MASK_6 = 0x000000000000FF00;
  uint64 constant MASK_7 = 0x00000000000000FF;

  uint64 constant SHIFT_0 = 0x0100000000000000;
  uint64 constant SHIFT_1 = 0x0000010000000000;
  uint64 constant SHIFT_2 = 0x0000000001000000;
  uint64 constant SHIFT_3 = 0x0000000000000100;

  struct BLAKE2b_ctx {
    uint256[4] b; //input buffer
    uint64[8] h;  //chained state
    uint128 t; //total bytes
    uint64 c; //Size of b
    uint outlen; //diigest output size
  }

  // Mixing Function
  function G(uint64[16] memory v, uint a, uint b, uint c, uint d, uint64 x, uint64 y) private pure {

       // Dereference to decrease memory reads
       uint64 va = v[a];
       uint64 vb = v[b];
       uint64 vc = v[c];
       uint64 vd = v[d];

       //Optimised mixing function
       assembly{
         // v[a] := (v[a] + v[b] + x) mod 2**64
         va := addmod(add(va,vb),x, 0x10000000000000000)
         //v[d] := (v[d] ^ v[a]) >>> 32
         //vd := xor(div(xor(vd,va), 0x100000000), mulmod(xor(vd, va),0x100000000, 0x10000000000000000))
         vd := xor(
                    shr(32, xor(vd,va)),
                    and(shl(32, xor(vd, va)), 0xffffffffffffffff)
                )
         //v[c] := (v[c] + v[d])     mod 2**64
         vc := addmod(vc,vd, 0x10000000000000000)
         //v[b] := (v[b] ^ v[c]) >>> 24
         //vb := xor(div(xor(vb,vc), 0x1000000), mulmod(xor(vb, vc),0x10000000000, 0x10000000000000000))
         vb := xor(
                    shr(24, xor(vb,vc)),
                    and(shl(40, xor(vb, vc)), 0xffffffffffffffff)
                    )
         
         // v[a] := (v[a] + v[b] + y) mod 2**64
         va := addmod(add(va,vb),y, 0x10000000000000000)
         //va := and(add(add(va,vb),y), 0xffffffffffffffff) more gas
         //v[d] := (v[d] ^ v[a]) >>> 16
         //vd := xor(div(xor(vd,va), 0x10000), mulmod(xor(vd, va),0x1000000000000, 0x10000000000000000))
         vd := xor(
                    shr(16, xor(vd,va)),
                    and(shl(48, xor(vd, va)), 0xffffffffffffffff)
                )
         //v[c] := (v[c] + v[d])     mod 2**64
         vc := addmod(vc,vd, 0x10000000000000000)
         //vc := and(add(vc,vd), 0xffffffffffffffff) more gas
         // v[b] := (v[b] ^ v[c]) >>> 63
         //vb := xor(div(xor(vb,vc), 0x8000000000000000), mulmod(xor(vb, vc),0x2, 0x10000000000000000))
         vb := xor(
                    shr(63, xor(vb,vc)),
                    and(shl(1, xor(vb, vc)), 0xffffffffffffffff)
                )
       }

       v[a] = va;
       v[b] = vb;
       v[c] = vc;
       v[d] = vd;
  }


  function compress(BLAKE2b_ctx memory ctx, bool last) private pure {
    //TODO: Look into storing these as uint256[4]
    uint64[16] memory v;
    uint64[16] memory m;

    uint64[8] memory IV = [
        0x6a09e667f3bcc908, 0xbb67ae8584caa73b,
        0x3c6ef372fe94f82b, 0xa54ff53a5f1d36f1,
        0x510e527fade682d1, 0x9b05688c2b3e6c1f,
        0x1f83d9abfb41bd6b, 0x5be0cd19137e2179
    ];


    for(uint i=0; i<8; i++){
      v[i] = ctx.h[i]; // v[:8] = h[:8]
      v[i+8] = IV[i];  // v[8:] = IV
    }

    v[12] = v[12] ^ uint64(ctx.t % 2**64);  //Lower word of t
    v[13] = v[13] ^ uint64(ctx.t / 2**64);

    if(last) v[14] = ~v[14];   //Finalization flag

    uint64 mi;  //Temporary stack variable to decrease memory ops
    uint b; // Input buffer

    for(uint8 i = 0; i <16; i++){ //Operate 16 words at a time
      uint k = i%4; //Current buffer word
      mi = 0;
      if(k == 0){
        b=ctx.b[i/4];  //Load relevant input into buffer
      }

      //Extract relevent input from buffer
      assembly{
        //mi := and(div(b,exp(2,mul(64,sub(3,k)))), 0xFFFFFFFFFFFFFFFF)
        mi := and(shr(mul(64,sub(3,k)),b),0xFFFFFFFFFFFFFFFF)
      }

      //Flip endianness
      m[i] = getWords(mi);
    }

    //Mix m

          G( v, 0, 4, 8, 12, m[0], m[1]);
          G( v, 1, 5, 9, 13, m[2], m[3]);
          G( v, 2, 6, 10, 14, m[4], m[5]);
          G( v, 3, 7, 11, 15, m[6], m[7]);
          G( v, 0, 5, 10, 15, m[8], m[9]);
          G( v, 1, 6, 11, 12, m[10], m[11]);
          G( v, 2, 7, 8, 13, m[12], m[13]);
          G( v, 3, 4, 9, 14, m[14], m[15]);


          G( v, 0, 4, 8, 12, m[14], m[10]);
          G( v, 1, 5, 9, 13, m[4], m[8]);
          G( v, 2, 6, 10, 14, m[9], m[15]);
          G( v, 3, 7, 11, 15, m[13], m[6]);
          G( v, 0, 5, 10, 15, m[1], m[12]);
          G( v, 1, 6, 11, 12, m[0], m[2]);
          G( v, 2, 7, 8, 13, m[11], m[7]);
          G( v, 3, 4, 9, 14, m[5], m[3]);


          G( v, 0, 4, 8, 12, m[11], m[8]);
          G( v, 1, 5, 9, 13, m[12], m[0]);
          G( v, 2, 6, 10, 14, m[5], m[2]);
          G( v, 3, 7, 11, 15, m[15], m[13]);
          G( v, 0, 5, 10, 15, m[10], m[14]);
          G( v, 1, 6, 11, 12, m[3], m[6]);
          G( v, 2, 7, 8, 13, m[7], m[1]);
          G( v, 3, 4, 9, 14, m[9], m[4]);


          G( v, 0, 4, 8, 12, m[7], m[9]);
          G( v, 1, 5, 9, 13, m[3], m[1]);
          G( v, 2, 6, 10, 14, m[13], m[12]);
          G( v, 3, 7, 11, 15, m[11], m[14]);
          G( v, 0, 5, 10, 15, m[2], m[6]);
          G( v, 1, 6, 11, 12, m[5], m[10]);
          G( v, 2, 7, 8, 13, m[4], m[0]);
          G( v, 3, 4, 9, 14, m[15], m[8]);


          G( v, 0, 4, 8, 12, m[9], m[0]);
          G( v, 1, 5, 9, 13, m[5], m[7]);
          G( v, 2, 6, 10, 14, m[2], m[4]);
          G( v, 3, 7, 11, 15, m[10], m[15]);
          G( v, 0, 5, 10, 15, m[14], m[1]);
          G( v, 1, 6, 11, 12, m[11], m[12]);
          G( v, 2, 7, 8, 13, m[6], m[8]);
          G( v, 3, 4, 9, 14, m[3], m[13]);


          G( v, 0, 4, 8, 12, m[2], m[12]);
          G( v, 1, 5, 9, 13, m[6], m[10]);
          G( v, 2, 6, 10, 14, m[0], m[11]);
          G( v, 3, 7, 11, 15, m[8], m[3]);
          G( v, 0, 5, 10, 15, m[4], m[13]);
          G( v, 1, 6, 11, 12, m[7], m[5]);
          G( v, 2, 7, 8, 13, m[15], m[14]);
          G( v, 3, 4, 9, 14, m[1], m[9]);


          G( v, 0, 4, 8, 12, m[12], m[5]);
          G( v, 1, 5, 9, 13, m[1], m[15]);
          G( v, 2, 6, 10, 14, m[14], m[13]);
          G( v, 3, 7, 11, 15, m[4], m[10]);
          G( v, 0, 5, 10, 15, m[0], m[7]);
          G( v, 1, 6, 11, 12, m[6], m[3]);
          G( v, 2, 7, 8, 13, m[9], m[2]);
          G( v, 3, 4, 9, 14, m[8], m[11]);


          G( v, 0, 4, 8, 12, m[13], m[11]);
          G( v, 1, 5, 9, 13, m[7], m[14]);
          G( v, 2, 6, 10, 14, m[12], m[1]);
          G( v, 3, 7, 11, 15, m[3], m[9]);
          G( v, 0, 5, 10, 15, m[5], m[0]);
          G( v, 1, 6, 11, 12, m[15], m[4]);
          G( v, 2, 7, 8, 13, m[8], m[6]);
          G( v, 3, 4, 9, 14, m[2], m[10]);


          G( v, 0, 4, 8, 12, m[6], m[15]);
          G( v, 1, 5, 9, 13, m[14], m[9]);
          G( v, 2, 6, 10, 14, m[11], m[3]);
          G( v, 3, 7, 11, 15, m[0], m[8]);
          G( v, 0, 5, 10, 15, m[12], m[2]);
          G( v, 1, 6, 11, 12, m[13], m[7]);
          G( v, 2, 7, 8, 13, m[1], m[4]);
          G( v, 3, 4, 9, 14, m[10], m[5]);


          G( v, 0, 4, 8, 12, m[10], m[2]);
          G( v, 1, 5, 9, 13, m[8], m[4]);
          G( v, 2, 6, 10, 14, m[7], m[6]);
          G( v, 3, 7, 11, 15, m[1], m[5]);
          G( v, 0, 5, 10, 15, m[15], m[11]);
          G( v, 1, 6, 11, 12, m[9], m[14]);
          G( v, 2, 7, 8, 13, m[3], m[12]);
          G( v, 3, 4, 9, 14, m[13], m[0]);


          G( v, 0, 4, 8, 12, m[0], m[1]);
          G( v, 1, 5, 9, 13, m[2], m[3]);
          G( v, 2, 6, 10, 14, m[4], m[5]);
          G( v, 3, 7, 11, 15, m[6], m[7]);
          G( v, 0, 5, 10, 15, m[8], m[9]);
          G( v, 1, 6, 11, 12, m[10], m[11]);
          G( v, 2, 7, 8, 13, m[12], m[13]);
          G( v, 3, 4, 9, 14, m[14], m[15]);


          G( v, 0, 4, 8, 12, m[14], m[10]);
          G( v, 1, 5, 9, 13, m[4], m[8]);
          G( v, 2, 6, 10, 14, m[9], m[15]);
          G( v, 3, 7, 11, 15, m[13], m[6]);
          G( v, 0, 5, 10, 15, m[1], m[12]);
          G( v, 1, 6, 11, 12, m[0], m[2]);
          G( v, 2, 7, 8, 13, m[11], m[7]);
          G( v, 3, 4, 9, 14, m[5], m[3]);



    //XOR current state with both halves of v
    for(uint8 i=0; i<8; ++i){
      ctx.h[i] = ctx.h[i] ^ v[i] ^ v[i+8];
    }

  }


  function init(BLAKE2b_ctx memory ctx, uint64 outlen, bytes memory key, uint64[2] memory salt, uint64[2] memory person) private pure {

      if(outlen == 0 || outlen > 64 || key.length > 64) revert();

      uint64[8] memory IV = [
          0x6a09e667f3bcc908, 0xbb67ae8584caa73b,
          0x3c6ef372fe94f82b, 0xa54ff53a5f1d36f1,
          0x510e527fade682d1, 0x9b05688c2b3e6c1f,
          0x1f83d9abfb41bd6b, 0x5be0cd19137e2179
      ];

      //Initialize chained-state to IV
      for(uint i = 0; i< 8; i++){
        ctx.h[i] = IV[i];
      }

      // Set up parameter block
      //ctx.h[0] = ctx.h[0] ^ 0x01010000 ^ shift_left(uint64(key.length), 8) ^ outlen;
      ctx.h[0] = ctx.h[0] ^ 0x01010000 ^ (uint64(key.length) << 8) ^ outlen;
      ctx.h[4] = ctx.h[4] ^ salt[0];
      ctx.h[5] = ctx.h[5] ^ salt[1];
      ctx.h[6] = ctx.h[6] ^ person[0];
      ctx.h[7] = ctx.h[7] ^ person[1];

      ctx.outlen = outlen;
      uint64 i = uint64(key.length);

      //Run hash once with key as input
      if(i > 0){
        update(ctx, key);
        ctx.c = 128;
      }
  }


  function update(BLAKE2b_ctx memory ctx, bytes memory input) private pure {

    for(uint i = 0; i < input.length; i++){
      //If buffer is full, update byte counters and compress
      if(ctx.c == 128){
        ctx.t += ctx.c;
        compress(ctx, false);
        ctx.c = 0;
      }

      //Update temporary counter c
      uint c = ctx.c++;

      // b -> ctx.b
      uint256[4] memory b = ctx.b;
      uint8 a = uint8(input[i]);

      // ctx.b[c] = a
      assembly{
        mstore8(add(b,c),a)
      }
    }
  }


  function finalize(BLAKE2b_ctx memory ctx, uint64[8] memory out) private pure {
    // Add any uncounted bytes
    ctx.t += ctx.c;
    
    // zero out left over bytes (if key is longer than input)
    uint c = ctx.c++;
    uint8 a = 0;
    uint256[4] memory b = ctx.b;
    for(uint i = c; i < 128; i++) {
      // ctx.b[i] = 0
      assembly{
        mstore8(add(b,i),a)
      }
    }

    // Compress with finalization flag
    compress(ctx,true);

    //Flip little to big endian and store in output buffer
    for(uint i=0; i < ctx.outlen / 8; i++){
      out[i] = getWords(ctx.h[i]);
    }

    //Properly pad output if it doesn't fill a full word
    if(ctx.outlen < 64){
      //out[ctx.outlen/8] = shift_right(getWords(ctx.h[ctx.outlen/8]),64-8*(ctx.outlen%8));
      out[ctx.outlen/8] = getWords(ctx.h[ctx.outlen/8]) >> (64-8*(ctx.outlen%8));
    }

  }

  //Helper function for full hash function
  function blake2b(bytes memory input, bytes memory key, bytes memory salt, bytes memory personalization, uint64 outlen) pure public returns(uint64[8] memory){

    BLAKE2b_ctx memory ctx;
    uint64[8] memory out;

    init(ctx, outlen, key, formatInput(salt), formatInput(personalization));
    update(ctx, input);
    finalize(ctx, out);
    return out;
  }

  function blake2b(bytes memory input, bytes memory key, uint64 outlen) pure public returns (uint64[8] memory){
    return blake2b(input, key, "", "", outlen);
  }

// Utility functions

  //Flips endianness of words
  function getWords(uint64 a) pure private returns (uint64 b) {
    return  (a & MASK_0) / SHIFT_0 ^
            (a & MASK_1) / SHIFT_1 ^
            (a & MASK_2) / SHIFT_2 ^
            (a & MASK_3) / SHIFT_3 ^
            (a & MASK_4) * SHIFT_3 ^
            (a & MASK_5) * SHIFT_2 ^
            (a & MASK_6) * SHIFT_1 ^
            (a & MASK_7) * SHIFT_0;
  }

  //bytes -> uint64[2]
  function formatInput(bytes memory input) pure private returns (uint64[2] memory output){
    for(uint i = 0; i<input.length; i++){
        //output[i/8] = output[i/8] ^ shift_left(uint64(input[i]), 64-8*(i%8+1));
        uint64 x;
        assembly {
            x := mload(add(input, add(0x08, i)))
        }
        //output[i/8] = output[i/8] ^ shift_left(x, 64-8*(i%8+1));
        output[i/8] = output[i/8] ^ (x << (64-8*(i%8+1)));
    }
        output[0] = getWords(output[0]);
        output[1] = getWords(output[1]);
  }

  function formatOutput(uint64[8] memory input) pure private returns (bytes32[2] memory){
    bytes32[2] memory result;

    for(uint i = 0; i < 8; i++){
        result[i/4] = result[i/4] ^ bytes32(input[i] * 2**(64*(3-i%4)));
    }
    return result;
  }
}

*/





func BenchmarkBlake2b_shift_optimized(b *testing.B) {
	var definition = `[{"constant":true,"inputs":[{"name":"input","type":"bytes"},{"name":"key","type":"bytes"},{"name":"salt","type":"bytes"},{"name":"personalization","type":"bytes"},{"name":"outlen","type":"uint64"}],"name":"blake2b","outputs":[{"name":"","type":"uint64[8]"}],"payable":false,"stateMutability":"pure","type":"function"},{"constant":true,"inputs":[{"name":"input","type":"bytes"},{"name":"key","type":"bytes"},{"name":"outlen","type":"uint64"}],"name":"blake2b","outputs":[{"name":"","type":"uint64[8]"}],"payable":false,"stateMutability":"pure","type":"function"}]`

	var code = common.Hex2Bytes("608060405234801561001057600080fd5b50600436106100365760003560e01c80631e0924231461003b578063d299dac0146102bb575b600080fd5b610282600480360360a081101561005157600080fd5b81019060208101813564010000000081111561006c57600080fd5b82018360208201111561007e57600080fd5b803590602001918460018302840111640100000000831117156100a057600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092959493602081019350359150506401000000008111156100f357600080fd5b82018360208201111561010557600080fd5b8035906020019184600183028401116401000000008311171561012757600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561017a57600080fd5b82018360208201111561018c57600080fd5b803590602001918460018302840111640100000000831117156101ae57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561020157600080fd5b82018360208201111561021357600080fd5b8035906020019184600183028401116401000000008311171561023557600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295505050903567ffffffffffffffff1691506103f49050565b604051808261010080838360005b838110156102a8578181015183820152602001610290565b5050505090500191505060405180910390f35b610282600480360360608110156102d157600080fd5b8101906020810181356401000000008111156102ec57600080fd5b8201836020820111156102fe57600080fd5b8035906020019184600183028401116401000000008311171561032057600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561037357600080fd5b82018360208201111561038557600080fd5b803590602001918460018302840111640100000000831117156103a757600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295505050903567ffffffffffffffff1691506104489050565b6103fc61156d565b61040461158d565b61040c61156d565b61042982858961041b8a610485565b6104248a610485565b610538565b61043382896106dc565b61043d828261079b565b979650505050505050565b61045061156d565b61047d848460206040519081016040528060008152506020604051908101604052806000815250866103f4565b949350505050565b61048d6115ca565b60005b82518110156104fb5760088184018101519067ffffffffffffffff82166007841660010182026040031b9084908404600281106104c957fe5b6020020151188360088404600281106104de57fe5b67ffffffffffffffff909216602092909202015250600101610490565b5061050d8160005b60200201516108cd565b67ffffffffffffffff168152610524816001610503565b67ffffffffffffffff166020820152919050565b67ffffffffffffffff84161580610559575060408467ffffffffffffffff16115b80610565575060408351115b1561056f57600080fd5b61057761156d565b506040805161010081018252676a09e667f3bcc908815267bb67ae8584caa73b6020820152673c6ef372fe94f82b9181019190915267a54ff53a5f1d36f1606082015267510e527fade682d16080820152679b05688c2b3e6c1f60a0820152671f83d9abfb41bd6b60c0820152675be0cd19137e217960e082015260005b600881101561063d5781816008811061060a57fe5b602002015187602001518260088110151561062157fe5b67ffffffffffffffff90921660209290920201526001016105f5565b50835160208781018051805167ffffffffffffffff94851660081b188918630101000018841690528551815160809081018051909218851690915286830151825160a0018051909118851690528551825160c00180519091188516905291850151905160e00180519091188316905286821690880152845190600090821611156106d3576106cb87866106dc565b608060608801525b50505050505050565b60005b815181101561079657826060015167ffffffffffffffff166080141561074057606083015160408401805167ffffffffffffffff9092169091016fffffffffffffffffffffffffffffffff16905261073883600061094f565b600060608401525b60608301805167ffffffffffffffff600182018116909252166107616115e5565b508351835160009085908590811061077557fe5b90602001015160f81c60f81b60f81c905080838301535050506001016106df565b505050565b60608201805160408401805167ffffffffffffffff8084169182016fffffffffffffffffffffffffffffffff1690925260019092011690915260006107de6115e5565b508351825b60808110156107f95782818301536001016107e3565b5061080585600161094f565b60005b60808601516008900481101561085457602086015161082c90826008811061050357fe5b85826008811061083857fe5b67ffffffffffffffff9092166020929092020152600101610808565b506040856080015110156108c65760808501516020860151600860078316810260400392610889929190046008811061050357fe5b67ffffffffffffffff16901c84600887608001518115156108a657fe5b04600881106108b157fe5b67ffffffffffffffff90921660209290920201525b5050505050565b600067010000000000000060ff8316026501000000000061ff00841602630100000062ff000085160261010063ff000000861681029064ff00000000871604630100000065ff00000000008816046501000000000066ff00000000000089160467010000000000000067ff000000000000008a16041818181818181892915050565b610957611604565b61095f611604565b61096761156d565b506040805161010081018252676a09e667f3bcc908815267bb67ae8584caa73b6020820152673c6ef372fe94f82b9181019190915267a54ff53a5f1d36f1606082015267510e527fade682d16080820152679b05688c2b3e6c1f60a0820152671f83d9abfb41bd6b60c0820152675be0cd19137e217960e082015260005b6008811015610a5f57602086015181600881106109fe57fe5b6020020151848260108110610a0f57fe5b67ffffffffffffffff9092166020929092020152818160088110610a2f57fe5b6020020151846008830160108110610a4357fe5b67ffffffffffffffff90921660209290920201526001016109e5565b506040850180516101808501805167ffffffffffffffff928316188216905290516101a085018051680100000000000000006fffffffffffffffffffffffffffffffff9093169290920490911890911690528315610acc576101c0830180511967ffffffffffffffff1690525b600080805b60108160ff161015610b56576000925060038116801515610b0c578851600460ff84160460ff16600481101515610b0457fe5b602002015192505b67ffffffffffffffff83826003036040021c169350610b2a846108cd565b8660ff841660108110610b3957fe5b67ffffffffffffffff909216602092909202015250600101610ad1565b50610b7985600060046008600c89845b60200201518a60015b60200201516113e8565b610b9685600160056009600d8960025b60200201518a6003610b6f565b610bb38560026006600a600e8960045b60200201518a6005610b6f565b610bd08560036007600b600f8960065b60200201518a6007610b6f565b610bed8560006005600a600f8960085b60200201518a6009610b6f565b610c0a8560016006600b600c89600a5b60200201518a600b610b6f565b610c2785600260076008600d89600c5b60200201518a600d610b6f565b610c4385600360046009600e89815b60200201518a600f610b6f565b610c6085600060046008600c89600e5b60200201518a600a610b6f565b610c7d85600160056009600d8960045b60200201518a6008610b6f565b610c918560026006600a600e896009610c36565b610cae8560036007600b600f89600d5b60200201518a6006610b6f565b610ccb8560006005600a600f8960015b60200201518a600c610b6f565b610ce88560016006600b600c8960005b60200201518a6002610b6f565b610cfc85600260076008600d89600b610bc3565b610d1085600360046009600e896005610b89565b610d2485600060046008600c89600b610c70565b610d4185600160056009600d89600c5b60200201518a6000610b6f565b610d558560026006600a600e896005610cdb565b610d688560036007600b600f8981610c1a565b610d848560006005600a600f89825b60200201518a600e610b6f565b610d988560016006600b600c896003610ca1565b610dab85600260076008600d8983610b66565b610dc785600360046009600e89825b60200201518a6004610b6f565b610ddb85600060046008600c896007610be0565b610def85600160056009600d896003610b66565b610e038560026006600a600e89600d610cbe565b610e168560036007600b600f8982610d77565b610e2a8560006005600a600f896002610ca1565b610e3e8560016006600b600c896005610c53565b610e5285600260076008600d896004610d34565b610e6685600360046009600e89600f610c70565b610e7a85600060046008600c896009610d34565b610e8d85600160056009600d8983610bc3565b610ea08560026006600a600e8984610dba565b610eb48560036007600b600f89600a610c36565b610ec88560006005600a600f89600e610b66565b610edb8560016006600b600c8982610cbe565b610eef85600260076008600d896006610c70565b610f0285600360046009600e8984610c1a565b610f1685600060046008600c896002610cbe565b610f2a85600160056009600d896006610c53565b610f3e8560026006600a600e896000610bfd565b610f528560036007600b600f896008610b89565b610f668560006005600a600f896004610c1a565b610f7a8560016006600b600c896007610ba6565b610f8e85600260076008600d89600f610d77565b610fa285600360046009600e896001610be0565b610fb585600060046008600c8981610ba6565b610fc885600160056009600d8984610c36565b610fdb8560026006600a600e8981610c1a565b610fef8560036007600b600f896004610c53565b6110028560006005600a600f8984610bc3565b6110158560016006600b600c8983610b89565b61102985600260076008600d896009610cdb565b61103d85600360046009600e896008610bfd565b61105185600060046008600c89600d610bfd565b61106585600160056009600d896007610d77565b6110798560026006600a600e89600c610b66565b61108c8560036007600b600f8984610be0565b61109f8560006005600a600f8983610d34565b6110b38560016006600b600c89600f610dba565b6110c685600260076008600d8982610ca1565b6110da85600360046009600e896002610c53565b6110ee85600060046008600c896006610c36565b61110285600160056009600d89600e610be0565b6111168560026006600a600e89600b610b89565b61112a8560036007600b600f896000610c70565b61113e8560006005600a600f89600c610cdb565b6111528560016006600b600c89600d610bc3565b61116685600260076008600d896001610dba565b61117a85600360046009600e89600a610ba6565b61118e85600060046008600c89600a610cdb565b6111a285600160056009600d896008610dba565b6111b68560026006600a600e896007610ca1565b6111ca8560036007600b600f896001610ba6565b6111dd8560006005600a600f8981610bfd565b6111f18560016006600b600c896009610d77565b61120585600260076008600d896003610cbe565b61121985600360046009600e89600d610d34565b61122c85600060046008600c8984610b66565b61124085600160056009600d896002610b89565b6112548560026006600a600e896004610ba6565b6112688560036007600b600f896006610bc3565b61127c8560006005600a600f896008610be0565b6112908560016006600b600c89600a610bfd565b6112a485600260076008600d89600c610c1a565b6112b785600360046009600e8981610c36565b6112cb85600060046008600c89600e610c53565b6112df85600160056009600d896004610c70565b6112f38560026006600a600e896009610c36565b6113078560036007600b600f89600d610ca1565b61131b8560006005600a600f896001610cbe565b61132f8560016006600b600c896000610cdb565b61134385600260076008600d89600b610bc3565b61135785600360046009600e896005610b89565b60005b60088160ff1610156113de578560ff60088301166010811061137857fe5b60200201518660ff83166010811061138c57fe5b602002015189602001518360ff166008811015156113a657fe5b6020020151181888602001518260ff166008811015156113c257fe5b67ffffffffffffffff909216602092909202015260010161135a565b5050505050505050565b60008787601081106113f657fe5b60200201519050600088876010811061140b57fe5b60200201519050600089876010811061142057fe5b6020020151905060008a876010811061143557fe5b6020020151905068010000000000000000868486010893508318602081811c91901b67ffffffffffffffff161868010000000000000000818308915067ffffffffffffffff82841860281b1682841860181c18925068010000000000000000858486010893508318601081901c60309190911b67ffffffffffffffff161868010000000000000000818308928318603f81901c60019190911b67ffffffffffffffff1618929150838b8b601081106114e957fe5b67ffffffffffffffff9092166020929092020152828b8a6010811061150a57fe5b67ffffffffffffffff9092166020929092020152818b896010811061152b57fe5b67ffffffffffffffff9092166020929092020152808b886010811061154c57fe5b67ffffffffffffffff90921660209290920201525050505050505050505050565b610100604051908101604052806008906020820280388339509192915050565b6101e0604051908101604052806115a26115e5565b81526020016115af61156d565b81526000602082018190526040820181905260609091015290565b60408051808201825290600290829080388339509192915050565b6080604051908101604052806004906020820280388339509192915050565b61020060405190810160405280601090602082028038833950919291505056fea165627a7a72305820a59dc9d098d29bacdd88cb50c25c96ed4ba3047fd46a5c6ecf57e447a3c699100029")

	abi, err := abi.JSON(strings.NewReader(definition))
	if err != nil {
		b.Fatal(err)
	}

	// test vectors: https://github.com/BLAKE2/BLAKE2/blob/master/testvectors/blake2b-kat.txt

	input := common.Hex2Bytes("{{input}}")
	expected := "{{expected}}"
	key := common.Hex2Bytes("")
	outlen := uint64(64)

	verifyinput, err := abi.Pack("blake2b", input, key, outlen)

	if err != nil {
		b.Fatal(err)
	}

	var cfg = new(Config)
	setDefaults(cfg)
	cfg.ChainConfig = &params.ChainConfig{
		ChainID:        big.NewInt(1),
		HomesteadBlock: new(big.Int),
		DAOForkBlock:   new(big.Int),
		DAOForkSupport: false,
		EIP150Block:    new(big.Int),
		EIP155Block:    new(big.Int),
		EIP158Block:    new(big.Int),
		ByzantiumBlock:  big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
	}
	cfg.State, _ = state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))

	var (
		address = common.BytesToAddress([]byte("contract"))
		vmenv   = NewEnv(cfg)
		sender  = vm.AccountRef(cfg.Origin)
	)

	cfg.State.CreateAccount(address)
	cfg.State.SetCode(address, code)

	var (
		ret  []byte
		exec_err  error
		leftOverGas uint64
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ret, leftOverGas, exec_err = vmenv.Call(sender, address, verifyinput, cfg.GasLimit, cfg.Value)
	}
	b.StopTimer()

	gasUsed := cfg.GasLimit - leftOverGas

	if exec_err != nil {
		b.Error(exec_err)
		return
	}

	ret_hex := common.Bytes2Hex(ret)

  // 00000000000000000000000000000000000000000000000033d0825dddf7ada90000000000000000000000000000000000000000000000009b0e7e307104ad07000000000000000000000000000000000000000000000000ca9cfd9692214f1500000000000000000000000000000000000000000000000061356315e784f3e5000000000000000000000000000000000000000000000000a17e364ae9dbb14c000000000000000000000000000000000000000000000000b2036df932b77f4b000000000000000000000000000000000000000000000000292761365fb328de0000000000000000000000000000000000000000000000007afdc6d8998f5fc1
	returned_digest := ret_hex[48:64] + ret_hex[112:128] + ret_hex[176:192] + ret_hex[240:256] + ret_hex[304:320] + ret_hex[368:384] + ret_hex[432:448] + ret_hex[496:512]

	if returned_digest != expected {
		b.Error(fmt.Sprintf("Expected %v, got %v", expected, returned_digest))
		return
	}
	fmt.Println("gasUsed:", gasUsed)

}