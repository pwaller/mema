#include <iostream>
#include <algorithm>
#include <vector>

#include <stdlib.h>

int main(int argc, char* argv[]) {
    
    int arr[100] = {0};
    
    size_t N = sizeof(arr)/sizeof(arr[0]);
    
    for (int i = 0; i < N; i++)
        arr[i] = random();
    
    //std::sort(arr, arr+N);
    
    for (int i = 0; i < N; i++)
    for (int j = i; j < N; j++) {
        if (arr[i] > arr[j]) {
            std::swap(arr[i], arr[j]);
        }
    }
    
    return 0;
}
