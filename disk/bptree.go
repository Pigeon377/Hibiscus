package disk

type BPTree struct {
	name string
	root *IndexBlock
}

func NewBPTree(name string) *BPTree {
	index, err := ReadIndexBlockFromDiskByBlockID(0, name)

	// root node unexist
	if err != nil {

		// block id equals zero
		// its parent will always pointer to root index node
		err := WriteIndexBlockToDiskByBlockID(
			NewIndexBlock(0, 0, 1, 0, make([]*KI, 0)),
			name)
		if err != nil {
			panic(err)
		}
		index_ := NewIndexBlock(1, 1, -1, 0, make([]*KI, 0))
		err = WriteIndexBlockToDiskByBlockID(
			index_,
			name)
		if err != nil {
			panic(err)
		}
		index__ := NewLeafBlock(0, -114514, -1, 0, make([]*KV, 0))
		index__.parentIndex = 1
		err = WriteLeafBlockToDiskByBlockID(
			index__,
			name)
		if err != nil {
			panic(err)
		}
		meta := &TableMeta{
			tableName:        name,
			nextLeafBlockID:  1,
			nextIndexBlockID: 2,
		}
		meta.WriteTableMeta()
		return &BPTree{
			name: name,
			root: index_,
		}
	} else {
		rootNode, err := ReadIndexBlockFromDiskByBlockID(index.parent, name)
		if err != nil {
			panic(err)
			return nil
		}
		return &BPTree{
			name: name,
			root: rootNode,
		}
	}
}

func (tree *BPTree) Query(key int64) (bool, []byte) {
	leaf := tree.searchLeafNode(key)
	ok, res := leaf.Get(key)
	if !ok {
		return false, nil
	} else {
		if checkValueIsEqualDeleteMark(res) {
			return false, nil
		} else {
			return true, res
		}
	}
}

func (tree *BPTree) QueryAll() []*KV {
	res := make([]*KV, 0)
	nextBoundID := int64(0)
	for nextBoundID != -1 {
		leaf, err := ReadLeafBlockFromDiskByBlockID(nextBoundID, tree.name)
		if err != nil {
			panic(err)
		}
		for _, kv := range leaf.KVs {
			if !checkValueIsEqualDeleteMark(kv.Value) {
				res = append(res, kv)
			}
		}
		nextBoundID = leaf.nextBlockID
	}
	return res
}

func (tree *BPTree) Insert(key int64, value []byte) bool {
	return tree.insertMethodImplement(key, value)
}

func (tree *BPTree) Delete(key int64) bool {
	return tree.insertMethodImplement(key, deleteOperationMarkBytes)
}

func (tree *BPTree) Update(key int64, value []byte) bool {
	return tree.insertMethodImplement(key, value)
}

func (tree *BPTree) searchLeafNode(key int64) *LeafBlock {
	cursor := tree.root
	// ???????????????????????????
	for !cursor.isLeafIndex() {
		// ???????????????
		rightBound := searchRightBoundFromIndexNode(key, cursor)
		if rightBound == -1 {
			panic("bug occurred")
			return nil
		}

		nextBlockID := cursor.KIs[rightBound].Index
		index, err := ReadIndexBlockFromDiskByBlockID(nextBlockID, tree.name)
		if err != nil {
			panic(err)
			return nil
		}
		cursor = index
	}
	rightBound := searchRightBoundFromIndexNode(key, cursor)
	if rightBound == -1 {
		if cursor.isRoot() {
			leaf, err := ReadLeafBlockFromDiskByBlockID(0, tree.name)
			if err != nil {
				return nil
			}
			return leaf
		} else {
			panic("Odd Bug!")
		}
	}
	nextBoundID := cursor.KIs[rightBound].Index
	leaf, err := ReadLeafBlockFromDiskByBlockID(nextBoundID, tree.name)
	if err != nil {
		panic(err)
		return nil
	}
	return leaf
}

func (tree *BPTree) insertMethodImplement(key int64, value []byte) bool {
	// ??????????????????????????????
	cursor := tree.root
	// empty index
	// so data will be put in first leaf block
	// ?????????????????????????????????????????????
	// ?????????????????????????????????
	if cursor.childrenSize == 0 {
		// has no any index
		// ????????????0????????????
		// ????????????0????????????
		leaf, err := ReadLeafBlockFromDiskByBlockID(0, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		// ?????????????????????????????????
		return tree.insertIntoLeafNodeAndWrite(key, value, leaf)

	} else {
		// ???????????????????????????
		leaf := tree.searchLeafNode(key)

		return tree.insertIntoLeafNodeAndWrite(key, value, leaf)
	}
	return false
}

func (tree *BPTree) insertIntoLeafNodeAndWrite(key int64, value []byte, leaf *LeafBlock) bool {
	// ???????????????????????????????????????
	oldMaxKey := leaf.maxKey
	// ????????????????????????
	ok := leaf.Put(key, value)
	// ?????????????????????
	if !ok {
		return false
	}
	// ??????????????????????????????????????????
	// ????????????
	if leaf.kvsSize > leafNodeBlockMaxSize {
		// ??????????????????
		// ???????????????
		// leaf1 ??? leaf ?????????????????????
		// ??????????????????
		leaf1, leaf2 := SplitLeafNodeBlock(leaf, tree.name)
		// ????????????????????????????????????
		err := WriteLeafBlockToDiskByBlockID(leaf1, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		err = WriteLeafBlockToDiskByBlockID(leaf2, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		// ???????????????????????????????????????ID
		// ?????????????????????
		index, err := ReadIndexBlockFromDiskByBlockID(leaf.parentIndex, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		// ????????????????????????????????????
		// ?????????????????????
		// ?????????????????????
		// ???O(n)???
		oldKey := tree.getIndexKeyByOffsetID(leaf1.id, index)
		// ??????????????????????????????????????????
		index.Delete(oldKey)
		// ???????????????????????????
		err = WriteIndexBlockToDiskByBlockID(index, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		// ??????????????????
		tree.insertIntoIndexNodeAndWrite(leaf1.maxKey, leaf1.id, index)
		tree.insertIntoIndexNodeAndWrite(leaf2.maxKey, leaf2.id, index)
		// ????????????????????????????????????
		// ??????????????????
		if index.isRoot() {
			// root node always stay in memory
			// must update it at once if its block message change
			tree.root = index

		}
		return true
	} else {
		// ???????????????
		err := WriteLeafBlockToDiskByBlockID(leaf, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		if key > oldMaxKey {
			index, err := ReadIndexBlockFromDiskByBlockID(leaf.parentIndex, tree.name)
			if err != nil {
				panic(err)
				return false
			}
			oldKey := tree.getIndexKeyByOffsetID(leaf.id, index)
			index.Delete(oldKey)
			err = WriteIndexBlockToDiskByBlockID(index, tree.name)
			if err != nil {
				panic(err)
				return false
			}
			tree.insertIntoIndexNodeAndWrite(key, leaf.id, index)
		}
		return true
	}
}

func (tree *BPTree) insertIntoIndexNodeAndWrite(key int64, blockID int64, index *IndexBlock) bool {
	// ????????????????????????
	ok := index.Put(key, blockID)
	if !ok {
		return false
	}
	// ???????????????
	err := WriteIndexBlockToDiskByBlockID(index, tree.name)
	if err != nil {
		panic(err)
		return false
	}
	// ???????????????????????????
	for !index.isRoot() {
		// ??????????????????
		index_, err := ReadIndexBlockFromDiskByBlockID(index.parent, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		// ??????????????????
		// ????????????
		if index.isFull() {
			index1, index2 := SplitIndexNodeBlock(index, tree.name)
			err := WriteIndexBlockToDiskByBlockID(
				index1,
				tree.name,
			)
			if err != nil {
				return false
			}
			err = WriteIndexBlockToDiskByBlockID(
				index2,
				tree.name,
			)
			if err != nil {
				return false
			}
			// remove old index which point to node before spilt
			// ??????????????????
			// ???????????????????????????
			oldKey := tree.getIndexKeyByOffsetID(index1.id, index_)
			index_.Delete(oldKey)
			index_.Put(index1.KIs[index1.childrenSize-1].Key, index1.id)
			index_.Put(index2.KIs[index2.childrenSize-1].Key, index2.id)

			//???????????????
			if index.isLeafIndex() {
				for _, ki := range index2.KIs {
					needUpdateLeaf, err := ReadLeafBlockFromDiskByBlockID(ki.Index, tree.name)
					if err != nil {
						panic(err)
						return false
					}
					needUpdateLeaf.parentIndex = index2.id
					err = WriteLeafBlockToDiskByBlockID(needUpdateLeaf, tree.name)
					if err != nil {
						panic(err)
						return false
					}
				}
			} else {
				for _, ki := range index2.KIs {
					needUpdateIndex, err := ReadIndexBlockFromDiskByBlockID(ki.Index, tree.name)
					if err != nil {
						panic(err)
						return false
					}
					needUpdateIndex.parent = index2.id
					err = WriteIndexBlockToDiskByBlockID(needUpdateIndex, tree.name)
					if err != nil {
						panic(err)
						return false
					}
				}
			}

		} else {
			// ????????????????????????
			// ??????????????????
			// ??????????????????key?????????
			// ????????????
			oldKey := tree.getIndexKeyByOffsetID(index.id, index_)
			if oldKey < key {
				if oldKey == -1 {
					panic("Illegal Block")
				} else {
					index_.Delete(oldKey)
					index_.Put(key, index.id)
					if index_.isRoot() {
						tree.root = index_
					}
				}
			}
		}
		// ??????????????????
		err = WriteIndexBlockToDiskByBlockID(index_, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		index = index_
		if index_.isRoot() {
			tree.root = index_
		}
	}
	if index.isFull() {
		index1, index2 := SplitIndexNodeBlock(index, tree.name)
		newRoot := NewIndexBlock(NextIndexNodeBlockID(tree.name), 0, -1, 0, make([]*KI, 0))
		index1.parent = newRoot.id
		index2.parent = newRoot.id
		newRoot.Put(index1.KIs[index1.childrenSize-1].Key, index1.id)
		newRoot.Put(index2.KIs[index2.childrenSize-1].Key, index2.id)
		tree.setRootNode(newRoot)
		err := WriteIndexBlockToDiskByBlockID(newRoot, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		err = WriteIndexBlockToDiskByBlockID(index1, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		err = WriteIndexBlockToDiskByBlockID(index2, tree.name)
		if err != nil {
			panic(err)
			return false
		}
		//???????????????
		if index.isLeafIndex() {
			for _, ki := range index2.KIs {
				needUpdateLeaf, err := ReadLeafBlockFromDiskByBlockID(ki.Index, tree.name)
				if err != nil {
					panic(err)
					return false
				}
				needUpdateLeaf.parentIndex = index2.id
				err = WriteLeafBlockToDiskByBlockID(needUpdateLeaf, tree.name)
				if err != nil {
					panic(err)
					return false
				}
			}
		} else {
			for _, ki := range index2.KIs {
				needUpdateIndex, err := ReadIndexBlockFromDiskByBlockID(ki.Index, tree.name)
				if err != nil {
					panic(err)
					return false
				}
				needUpdateIndex.parent = index2.id
				err = WriteIndexBlockToDiskByBlockID(needUpdateIndex, tree.name)
				if err != nil {
					panic(err)
					return false
				}
			}
		}
		index = newRoot
	}
	return true
}

func (tree *BPTree) setRootNode(newRootIndex *IndexBlock) {
	root := NewIndexBlock(0, 0, newRootIndex.id, 0, make([]*KI, 0))
	err := WriteIndexBlockToDiskByBlockID(root, tree.name)
	if err != nil {
		panic(err)
	}
	tree.root = newRootIndex
}

func (tree *BPTree) getIndexKeyByOffsetID(offset int64, index *IndexBlock) int64 {
	for _, ki := range index.KIs {
		if ki.Index == offset {
			return ki.Key
		}
	}
	return -1
}

func checkValueIsEqualDeleteMark(value []byte) bool {
	if len(value) != len(deleteOperationMarkBytes) {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] != deleteOperationMarkBytes[i] {
			return false
		}
	}
	return true
}

func getRootNode(tableName string) int64 {
	index, err := ReadIndexBlockFromDiskByBlockID(0, tableName)
	if err != nil {
		panic(err)
	}
	return index.parent
}

func searchRightBoundFromIndexNode(key int64, index *IndexBlock) int64 {
	if len(index.KIs) == 0 {
		return -1
	}
	left := int64(0)
	right := index.childrenSize
	for left < right {
		mid := (left + right) >> 1
		if index.KIs[mid].Key >= key {
			right = mid
		} else {
			left = mid + 1
		}
	}

	if left == int64(len(index.KIs)) && left != 0 {
		return left - 1
	}
	return left
}
