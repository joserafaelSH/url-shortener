use sha2::{Digest, Sha256};


const BASE62: &[u8] = b"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"; 
const BASE: u64 = 62;
const MAX_LENGTH: usize = 7;

fn hash_url(url: &str, unique_id: u64) -> String {
    let new_url = format!("{}:{}", url, unique_id); 
    let mut sha256 = Sha256::new(); 
    sha256.update(new_url.as_bytes());
    let hash = sha256.finalize();
    let num = u64::from_be_bytes(hash[0..8].try_into().unwrap());
    let hashed_url = to_base62(num);
    return hashed_url;
}

fn to_base62(mut num: u64) -> String {
    let mut result = [b'0'; MAX_LENGTH];

    for i in (0..MAX_LENGTH).rev() {
        result[i] = BASE62[(num % BASE) as usize];
        num /= BASE;
    }

    String::from_utf8(result.to_vec()).unwrap()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hash_url() {
        let url = "https://www.example.com";
        let unique_id = 12345;
        let hashed = hash_url(url, unique_id);
        assert_eq!(hashed.len(), 7);
    }   


    
    #[test]
    fn test_hash_url_different_ids() {
        let url = "https://www.example.com";
        let hash1 = hash_url(url, 1);
        let hash2 = hash_url(url, 2);
        assert_ne!(hash1, hash2);
    }
}