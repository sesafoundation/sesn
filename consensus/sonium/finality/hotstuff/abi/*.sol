// ValidatorRegistry.sol (interface snippet)
pragma solidity ^0.8.20;
interface ValidatorRegistry {
    function activeValidators() external view returns (address[] memory addrs, bytes[] memory blsPubkeys);
}

// Slash.sol (interface snippet)
pragma solidity ^0.8.20;
interface Slash {
    // Report equivocation: same height/view, different blockId by 'voter'
    function reportEquivocation(
        uint64 height,
        uint64 view,
        bytes32 blockIdA,
        bytes32 blockIdB,
        address voter,
        bytes calldata proofA, // sig/proof payloads
        bytes calldata proofB
    ) external;
}
